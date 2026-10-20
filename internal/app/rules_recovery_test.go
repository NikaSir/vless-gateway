package app

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestStartupRecoversRulesWithoutLiveReload(t *testing.T) {
	root := t.TempDir()
	rr := filepath.Join(root, "rules")
	old := filepath.Join(rr, "generations", "old")
	os.MkdirAll(old, 0700)
	os.WriteFile(filepath.Join(old, "manifest.json"), []byte(`{"generation":"old","files":[]}`), 0600)
	os.Symlink("generations/candidate", filepath.Join(rr, "current"))
	os.WriteFile(filepath.Join(rr, "transaction.json"), []byte(`{"previous":"generations/old","next":"generations/candidate"}`), 0600)
	if e := RunCLI(context.Background(), []string{"--root", root, "recover-rules"}, io.Discard, 0); e != nil {
		t.Fatal(e)
	}
	link, e := os.Readlink(filepath.Join(rr, "current"))
	if e != nil || link != "generations/old" {
		t.Fatal("unconfirmed candidate survived startup", link, e)
	}
	if _, e = os.Stat(filepath.Join(rr, "transaction.json")); !os.IsNotExist(e) {
		t.Fatal("recovery not committed", e)
	}
	if e := RunCLI(context.Background(), []string{"--root", root, "recover-rules"}, io.Discard, 0); e != nil {
		t.Fatal("recovery not idempotent", e)
	}
	if e := RunCLI(context.Background(), []string{"--root", root, "recover-rules"}, io.Discard, 1000); e == nil {
		t.Fatal("nonroot recovery allowed")
	}
}
