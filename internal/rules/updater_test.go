package rules

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

func TestFailedRollbackRetainsRecoveryJournal(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "generations", "old")
	os.MkdirAll(old, 0700)
	os.WriteFile(filepath.Join(old, "manifest.json"), []byte(`{"generation":"old","files":[]}`), 0600)
	os.Symlink("generations/old", filepath.Join(root, "current"))
	u := Updater{Root: root, Fetcher: fetch{}, Tool: tool{}, Activator: activate{true}}
	_, e := u.Update(context.Background())
	if e == nil || strings.Contains(e.Error(), "previous restored") {
		t.Fatal("unacknowledged rollback claimed restored", e)
	}
	if _, e = os.Stat(filepath.Join(root, "transaction.json")); e != nil {
		t.Fatal("rollback lost recovery journal", e)
	}
}

type interruptedActivation struct{}

func (interruptedActivation) ReloadAndCheck(context.Context) error {
	panic("power loss before acknowledgement")
}
func TestDiskRecoveryAfterInterruptedActivation(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "generations", "old")
	os.MkdirAll(old, 0700)
	os.WriteFile(filepath.Join(old, "manifest.json"), []byte(`{"generation":"old","files":[]}`), 0600)
	os.Symlink("generations/old", filepath.Join(root, "current"))
	u := Updater{Root: root, Fetcher: fetch{}, Tool: tool{}, Activator: interruptedActivation{}}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected interruption")
			}
		}()
		u.Update(context.Background())
	}()
	link, _ := os.Readlink(filepath.Join(root, "current"))
	if link == "generations/old" {
		t.Fatal("candidate not published before interruption")
	}
	if e := RecoverDisk(root); e != nil {
		t.Fatal(e)
	}
	link, _ = os.Readlink(filepath.Join(root, "current"))
	if link != "generations/old" {
		t.Fatal("unconfirmed candidate survived restart", link)
	}
	if _, e := os.Stat(filepath.Join(root, "transaction.json")); !os.IsNotExist(e) {
		t.Fatal("recovery journal not cleared", e)
	}
}

type breakRollback struct{ root string }

func (a breakRollback) ReloadAndCheck(context.Context) error {
	os.Remove(filepath.Join(a.root, "current"))
	os.Mkdir(filepath.Join(a.root, "current"), 0700)
	return errors.New("activation failed")
}
func TestFailedRollbackSwitchRetainsRecoveryJournal(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "generations", "old")
	os.MkdirAll(old, 0700)
	os.WriteFile(filepath.Join(old, "manifest.json"), []byte(`{"generation":"old","files":[]}`), 0600)
	os.Symlink("generations/old", filepath.Join(root, "current"))
	u := Updater{Root: root, Fetcher: fetch{}, Tool: tool{}, Activator: breakRollback{root}}
	_, e := u.Update(context.Background())
	if e == nil || !strings.Contains(e.Error(), "rollback switch failed") {
		t.Fatal("rollback failure not reported", e)
	}
	if _, e = os.Stat(filepath.Join(root, "transaction.json")); e != nil {
		t.Fatal("lost recovery journal", e)
	}
}
func TestDiskRecoveryRejectsCorruptOrMissingKnownGood(t *testing.T) {
	for _, journal := range []string{`{`, `{"previous":"../escape","next":"generations/bad"}`, `{"previous":"generations/missing","next":"generations/bad"}`} {
		root := t.TempDir()
		os.Symlink("generations/bad", filepath.Join(root, "current"))
		os.WriteFile(filepath.Join(root, "transaction.json"), []byte(journal), 0600)
		if e := RecoverDisk(root); e == nil {
			t.Fatal("unsafe recovery accepted")
		}
		link, _ := os.Readlink(filepath.Join(root, "current"))
		if link != "generations/bad" {
			t.Fatal("recovery switched despite invalid known-good")
		}
		if _, e := os.Stat(filepath.Join(root, "transaction.json")); e != nil {
			t.Fatal("lost journal", e)
		}
	}
}

type firstActivationFails struct{ calls int }

func (a *firstActivationFails) ReloadAndCheck(context.Context) error {
	a.calls++
	if a.calls == 1 {
		return errors.New("candidate failed")
	}
	return nil
}
func TestAcknowledgedRollbackClearsJournal(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "generations", "old")
	os.MkdirAll(old, 0700)
	os.WriteFile(filepath.Join(old, "manifest.json"), []byte(`{"generation":"old","files":[]}`), 0600)
	os.Symlink("generations/old", filepath.Join(root, "current"))
	a := &firstActivationFails{}
	u := Updater{Root: root, Fetcher: fetch{}, Tool: tool{}, Activator: a}
	_, e := u.Update(context.Background())
	if e == nil || !strings.Contains(e.Error(), "previous restored") {
		t.Fatal("acknowledged restoration not reported", e)
	}
	if _, e = os.Stat(filepath.Join(root, "transaction.json")); !os.IsNotExist(e) {
		t.Fatal("acknowledged rollback journal remains", e)
	}
	link, _ := os.Readlink(filepath.Join(root, "current"))
	if link != "generations/old" {
		t.Fatal("known good not restored", link)
	}
}
func TestDiskRecoveryCannotInterruptLiveTransaction(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "transaction.json"), []byte(`{"previous":"generations/old","next":"generations/new"}`), 0600)
	lock, e := os.OpenFile(filepath.Join(root, ".update.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		t.Fatal(e)
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		t.Fatal(e)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	if e = RecoverDisk(root); e == nil || e.Error() != "rule update already active" {
		t.Fatal("live transaction recovery not refused", e)
	}
	if _, e = os.Stat(filepath.Join(root, "transaction.json")); e != nil {
		t.Fatal("journal lost during active transaction", e)
	}
}
