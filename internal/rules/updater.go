package rules

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/NikaSir/vless-gateway/internal/state"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

const MaxRuleBytes = 32 << 20

type Fetcher interface {
	Fetch(context.Context, string, io.Writer) (FetchMeta, error)
}
type Tool interface {
	ValidateBinary(context.Context, string) error
	CompileSource(context.Context, string, string) error
	CheckConfig(context.Context, string) error
}
type Activator interface{ ReloadAndCheck(context.Context) error }
type Updater struct {
	Root, Overrides string
	Sources         map[string]string
	Fetcher         Fetcher
	Tool            Tool
	Activator       Activator
}
type HTTPFetcher struct{ Client *http.Client }

func (f HTTPFetcher) Fetch(ctx context.Context, raw string, w io.Writer) (FetchMeta, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Host != "raw.githubusercontent.com" || u.User != nil {
		return FetchMeta{}, errors.New("rule source not allowed")
	}
	h := f.Client
	if h == nil {
		h = &http.Client{Timeout: 60 * time.Second}
	}
	hc := *h
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, e := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if e != nil {
		return FetchMeta{}, errors.New("invalid rule request")
	}
	resp, e := hc.Do(req)
	if e != nil {
		return FetchMeta{}, errors.New("rule download unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return FetchMeta{}, errors.New("rule download rejected")
	}
	n, e := io.Copy(w, io.LimitReader(resp.Body, MaxRuleBytes+1))
	if e != nil || n == 0 || n > MaxRuleBytes {
		return FetchMeta{}, errors.New("invalid rule download size")
	}
	return FetchMeta{n}, nil
}

type journal struct {
	Previous string `json:"previous"`
	Next     string `json:"next"`
}

func safeGeneration(s string) bool {
	return strings.HasPrefix(s, "generations/") && filepath.Clean(s) == s && len(strings.Split(s, "/")) == 2
}
func switchLink(root, target string) error {
	if !safeGeneration(target) {
		return errors.New("invalid rule generation target")
	}
	tmp := filepath.Join(root, ".current-next")
	_ = os.Remove(tmp)
	if e := os.Symlink(target, tmp); e != nil {
		return e
	}
	if e := os.Rename(tmp, filepath.Join(root, "current")); e != nil {
		return e
	}
	dir, e := os.Open(root)
	if e != nil {
		return e
	}
	defer dir.Close()
	return dir.Sync()
}
func (u *Updater) Update(ctx context.Context) (Manifest, error) {
	var m Manifest
	if u.Fetcher == nil || u.Tool == nil || u.Activator == nil {
		return m, errors.New("rule updater unconfigured")
	}
	if e := os.MkdirAll(filepath.Join(u.Root, "generations"), 0700); e != nil {
		return m, e
	}
	lock, e := os.OpenFile(filepath.Join(u.Root, ".update.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return m, e
	}
	defer lock.Close()
	if syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		return m, errors.New("rule update already active")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	jp := filepath.Join(u.Root, "transaction.json")
	if b, e := os.ReadFile(jp); e == nil {
		var j journal
		if json.Unmarshal(b, &j) != nil || !safeGeneration(j.Previous) {
			return m, errors.New("rule recovery journal corrupt")
		}
		if switchLink(u.Root, j.Previous) != nil {
			return m, errors.New("rule recovery failed")
		}
		if u.Activator.ReloadAndCheck(ctx) != nil {
			return m, errors.New("rule recovery reload failed")
		}
		_ = os.Remove(jp)
	} else if !os.IsNotExist(e) {
		return m, errors.New("rule recovery journal unreadable")
	}
	previous, e := os.Readlink(filepath.Join(u.Root, "current"))
	if e != nil || !safeGeneration(previous) {
		return m, errors.New("known-good rule generation required")
	}
	prior, e := os.ReadFile(filepath.Join(u.Root, previous, "manifest.json"))
	var pm Manifest
	if e != nil || json.Unmarshal(prior, &pm) != nil || pm.Generation == "" {
		return m, errors.New("current manifest corrupt")
	}
	m.Generation = time.Now().UTC().Format("20060102T150405.000000000")
	stage, e := os.MkdirTemp(filepath.Join(u.Root, "generations"), ".staging-")
	if e != nil {
		return m, e
	}
	defer os.RemoveAll(stage)
	sources := u.Sources
	if len(sources) == 0 {
		sources = map[string]string{"geosite-ru": "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-category-ru.srs", "geoip-ru": "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-ru.srs"}
	}
	for _, tag := range []string{"geosite-ru", "geoip-ru"} {
		path := filepath.Join(stage, tag+".srs")
		f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return m, e
		}
		_, e = u.Fetcher.Fetch(ctx, sources[tag], f)
		syncErr := f.Sync()
		_ = f.Close()
		if e != nil || syncErr != nil {
			return m, errors.New("rule download failed")
		}
		if u.Tool.ValidateBinary(ctx, path) != nil {
			return m, errors.New("rule binary invalid")
		}
		if e = appendFile(&m, path, tag, sources[tag]); e != nil {
			return m, e
		}
	}
	for _, tag := range []string{"force-direct", "force-vless"} {
		src := filepath.Join(u.Overrides, tag+".json")
		if u.Overrides == "" {
			src = filepath.Join(stage, tag+".json")
			if state.AtomicWrite(src, []byte(`{"version":3,"rules":[]}`), 0600) != nil {
				return m, errors.New("override staging failed")
			}
		}
		path := filepath.Join(stage, tag+".srs")
		if u.Tool.CompileSource(ctx, src, path) != nil {
			return m, errors.New("override compilation failed")
		}
		if e = appendFile(&m, path, tag, "local"); e != nil {
			return m, e
		}
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	if state.AtomicWrite(filepath.Join(stage, "manifest.json"), b, 0600) != nil {
		return m, errors.New("rule manifest write failed")
	}
	if u.Tool.CheckConfig(ctx, stage) != nil {
		return m, errors.New("full rule configuration invalid")
	}
	next := "generations/" + m.Generation
	dest := filepath.Join(u.Root, next)
	if os.Rename(stage, dest) != nil {
		return m, errors.New("rule generation publication failed")
	}
	b, _ = json.Marshal(journal{previous, next})
	if state.AtomicWrite(jp, b, 0600) != nil {
		return m, errors.New("rule journal write failed")
	}
	if switchLink(u.Root, next) != nil {
		return m, errors.New("rule switch failed")
	}
	if u.Activator.ReloadAndCheck(ctx) != nil {
		_ = switchLink(u.Root, previous)
		rc, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = u.Activator.ReloadAndCheck(rc)
		_ = os.Remove(jp)
		return m, errors.New("rule activation failed; previous restored")
	}
	_ = os.Remove(jp)
	u.collect(next, previous)
	return m, nil
}
func appendFile(m *Manifest, path, name, source string) error {
	b, e := os.ReadFile(path)
	if e != nil || len(b) == 0 || len(b) > MaxRuleBytes {
		return errors.New("invalid staged rule size")
	}
	sum := sha256.Sum256(b)
	m.Files = append(m.Files, File{name, source, hex.EncodeToString(sum[:]), int64(len(b)), time.Now().UTC()})
	return nil
}
func (u *Updater) collect(current, previous string) {
	entries, _ := os.ReadDir(filepath.Join(u.Root, "generations"))
	var names []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	kept := 0
	for _, n := range names {
		rel := "generations/" + n
		if rel == current || rel == previous {
			continue
		}
		kept++
		if kept > 1 {
			_ = os.RemoveAll(filepath.Join(u.Root, rel))
		}
	}
}
