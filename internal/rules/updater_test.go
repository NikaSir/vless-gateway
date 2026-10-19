package rules

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fetch struct{ bad bool }

func (f fetch) Fetch(ctx context.Context, url string, w io.Writer) (FetchMeta, error) {
	if strings.HasSuffix(url, "/geosite-ru.srs") {
		return FetchMeta{}, errors.New("upstream filename does not exist")
	}
	if f.bad {
		return FetchMeta{}, errors.New("failed")
	}
	n, e := io.WriteString(w, "validated-binary")
	return FetchMeta{Bytes: int64(n)}, e
}

type tool struct{ fail string }

func (t tool) ValidateBinary(context.Context, string) error {
	if t.fail == "binary" {
		return errors.New("bad")
	}
	return nil
}
func (t tool) CompileSource(_ context.Context, src, dst string) error {
	if t.fail == "compile" {
		return errors.New("bad")
	}
	return os.WriteFile(dst, []byte("compiled"), 0600)
}
func (t tool) CheckConfig(context.Context, string) error {
	if t.fail == "config" {
		return errors.New("bad")
	}
	return nil
}

type activate struct{ fail bool }

func (a activate) ReloadAndCheck(context.Context) error {
	if a.fail {
		return errors.New("bad")
	}
	return nil
}
func TestUpdateNeverLosesKnownGood(t *testing.T) {
	for _, failure := range []string{"download", "binary", "compile", "config", "reload", ""} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			old := filepath.Join(root, "generations", "old")
			_ = os.MkdirAll(old, 0700)
			_ = os.WriteFile(filepath.Join(old, "manifest.json"), []byte(`{"generation":"old","files":[]}`), 0600)
			_ = os.Symlink("generations/old", filepath.Join(root, "current"))
			u := Updater{Root: root, Fetcher: fetch{failure == "download"}, Tool: tool{failure}, Activator: activate{failure == "reload"}}
			m, e := u.Update(context.Background())
			link, _ := os.Readlink(filepath.Join(root, "current"))
			if failure != "" {
				if e == nil || link != "generations/old" {
					t.Fatal("known good lost", link, e)
				}
			} else {
				if e != nil || !strings.Contains(link, m.Generation) || len(m.Files) != 4 {
					t.Fatal(m, link, e)
				}
			}
			if _, e = os.Stat(old); e != nil {
				t.Fatal("previous generation deleted")
			}
		})
	}
}
func TestUpdateRecoversInterruptedPublication(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "generations", "old"), 0700)
	_ = os.WriteFile(filepath.Join(root, "generations", "old", "manifest.json"), []byte(`{"generation":"old","files":[]}`), 0600)
	_ = os.Symlink("generations/bad", filepath.Join(root, "current"))
	_ = os.WriteFile(filepath.Join(root, "transaction.json"), []byte(`{"previous":"generations/old","next":"generations/bad"}`), 0600)
	u := Updater{Root: root, Fetcher: fetch{true}, Tool: tool{}, Activator: activate{}}
	_, e := u.Update(context.Background())
	link, _ := os.Readlink(filepath.Join(root, "current"))
	if e == nil || link != "generations/old" {
		t.Fatal("crash recovery failed", link, e)
	}
}
func TestFetcherRefusesUntrustedSources(t *testing.T) {
	f := HTTPFetcher{}
	for _, url := range []string{"http://raw.githubusercontent.com/a", "https://evil.example/a", "https://user:pass@raw.githubusercontent.com/a"} {
		if _, e := f.Fetch(context.Background(), url, io.Discard); e == nil {
			t.Fatal("untrusted source accepted")
		}
	}
}
