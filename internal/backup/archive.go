package backup

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/NikaSir/vless-gateway/internal/state"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var Paths = []string{"etc/config/network", "etc/config/firewall", "etc/config/dhcp", "etc/config/dropbear", "etc/config/nikas-vless", "etc/nikas-vless", "etc/init.d/nikas-sing-box", "etc/init.d/nikas-gatewayd", "etc/hotplug.d/iface/90-nikas-vless", "etc/sysctl.d/90-nikas-vless.conf", "usr/share/nftables.d/ruleset-post/90-nikas-vless.nft", "usr/bin/gatewayd", "usr/bin/gatewayctl", "usr/libexec/nikas-interception-cleanup", "usr/libexec/nikas-sing-box-run", "usr/libexec/nikas-vless", "etc/crontabs/root"}

type metadata struct {
	Format    int       `json:"format"`
	Board     string    `json:"board"`
	OS        string    `json:"os"`
	CreatedAt time.Time `json:"created_at"`
}

func allowed(path string) bool {
	if filepath.IsAbs(path) || filepath.Clean(path) != path || strings.Contains(path, "\\") {
		return false
	}
	if strings.HasPrefix(path, "etc/nikas-vless/backups") {
		return false
	}
	for _, p := range Paths {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}
func validLink(name, target string) bool {
	return name == "etc/nikas-vless/rules/current" && strings.HasPrefix(target, "generations/") && filepath.Clean(target) == target && len(strings.Split(target, "/")) == 2
}
func Create(root, out string) (string, error) {
	if !filepath.IsAbs(root) || !filepath.IsAbs(out) {
		return "", errors.New("absolute backup paths required")
	}
	if e := os.MkdirAll(out, 0700); e != nil {
		return "", e
	}
	path := filepath.Join(out, "nanopi-r5s-"+time.Now().UTC().Format("20060102T150405.000000000")+".tar.gz")
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return "", e
	}
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(path)
		}
	}()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	meta, _ := json.Marshal(metadata{1, "friendlyarm,nanopi-r5s", "FriendlyWrt 24.10.2", time.Now().UTC()})
	if tw.WriteHeader(&tar.Header{Name: ".nikas-backup.json", Mode: 0600, Size: int64(len(meta))}) != nil {
		return "", errors.New("backup header failed")
	}
	if _, e = tw.Write(meta); e != nil {
		return "", e
	}
	for _, p := range Paths {
		base := filepath.Join(root, p)
		if _, e = os.Lstat(base); os.IsNotExist(e) {
			continue
		}
		e = filepath.WalkDir(base, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, e := filepath.Rel(root, path)
			if e != nil {
				return e
			}
			if !allowed(rel) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return errors.New("unsupported backup entry")
			}
			info, e := d.Info()
			if e != nil {
				return e
			}
			link := ""
			if info.Mode()&os.ModeSymlink != 0 {
				link, e = os.Readlink(path)
				if e != nil || !validLink(rel, link) {
					return errors.New("unsafe backup symlink")
				}
			}
			header, e := tar.FileInfoHeader(info, link)
			if e != nil {
				return e
			}
			header.Name = rel
			header.Uid = 0
			header.Gid = 0
			header.Uname = ""
			header.Gname = ""
			if e = tw.WriteHeader(header); e != nil {
				return e
			}
			if info.Mode().IsRegular() {
				if info.Size() > 64<<20 {
					return errors.New("backup file too large")
				}
				file, e := os.Open(path)
				if e != nil {
					return e
				}
				_, e = io.Copy(tw, file)
				_ = file.Close()
				return e
			}
			return nil
		})
		if e != nil {
			return "", e
		}
	}
	if tw.Close() != nil || gz.Close() != nil || f.Sync() != nil {
		return "", errors.New("backup flush failed")
	}
	_ = f.Close()
	data, e := os.ReadFile(path)
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(data)
	if state.AtomicWrite(path+".sha256", []byte(hex.EncodeToString(sum[:])+"\n"), 0600) != nil {
		return "", errors.New("backup checksum write failed")
	}
	ok = true
	return path, nil
}

type entry struct {
	name, link string
	mode       os.FileMode
	data       []byte
	directory  bool
}

func Restore(root, path string) error {
	data, e := os.ReadFile(path)
	if e != nil || len(data) > 256<<20 {
		return errors.New("backup unreadable or oversized")
	}
	hash, e := os.ReadFile(path + ".sha256")
	if e != nil {
		return errors.New("backup checksum missing")
	}
	sum := sha256.Sum256(data)
	if strings.TrimSpace(string(hash)) != hex.EncodeToString(sum[:]) {
		return errors.New("backup checksum mismatch")
	}
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	gz, e := gzip.NewReader(f)
	if e != nil {
		return errors.New("invalid backup compression")
	}
	defer gz.Close()
	tr := tar.NewReader(io.LimitReader(gz, 256<<20+1))
	var entries []entry
	metaOK := false
	seen := map[string]bool{}
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return errors.New("invalid backup archive")
		}
		if seen[h.Name] {
			return errors.New("duplicate backup path")
		}
		seen[h.Name] = true
		if h.Name == ".nikas-backup.json" {
			if h.Size > 4096 {
				return errors.New("invalid backup metadata")
			}
			b, e := io.ReadAll(tr)
			var m metadata
			if e != nil || json.Unmarshal(b, &m) != nil || m.Format != 1 || m.Board != "friendlyarm,nanopi-r5s" || m.OS != "FriendlyWrt 24.10.2" {
				return errors.New("backup hardware/version mismatch")
			}
			metaOK = true
			continue
		}
		if !allowed(h.Name) || h.Size > 64<<20 {
			return errors.New("unsafe backup entry")
		}
		en := entry{name: h.Name, mode: os.FileMode(h.Mode).Perm()}
		switch h.Typeflag {
		case tar.TypeDir:
			en.directory = true
		case tar.TypeReg:
			b, e := io.ReadAll(tr)
			if e != nil {
				return errors.New("backup read failed")
			}
			en.data = b
		case tar.TypeSymlink:
			if !validLink(h.Name, h.Linkname) {
				return errors.New("unsafe backup symlink")
			}
			en.link = h.Linkname
		default:
			return errors.New("unsupported backup entry type")
		}
		entries = append(entries, en)
	}
	if !metaOK {
		return errors.New("backup metadata missing")
	}
	// Validation above is complete before the first write into the destination.
	for _, en := range entries {
		target := filepath.Join(root, en.name)
		if en.directory {
			if e = os.MkdirAll(target, en.mode); e != nil {
				return errors.New("restore directory failed")
			}
			continue
		}
		if en.link != "" {
			tmp := target + ".restore-link"
			_ = os.Remove(tmp)
			if os.Symlink(en.link, tmp) != nil || os.Rename(tmp, target) != nil {
				return errors.New("restore link failed")
			}
			continue
		}
		if e = state.AtomicWrite(target, en.data, en.mode); e != nil {
			return errors.New("restore file failed")
		}
	}
	return nil
}
