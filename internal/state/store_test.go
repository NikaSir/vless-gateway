package state

import (
	"context"
	"github.com/NikaSir/vless-gateway/internal/model"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreAtomicPermissionsAndCorruption(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	want := model.StatusSnapshot{FailoverSnapshot: model.FailoverSnapshot{Mode: model.ModePreferNetherlands}}
	if err := s.Save(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load(context.Background())
	if err != nil || got.Mode != want.Mode {
		t.Fatal(got, err)
	}
	info, _ := os.Stat(filepath.Join(root, "status.json"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("unsafe mode")
	}
	_ = os.WriteFile(filepath.Join(root, "status.json"), []byte("corrupt"), 0600)
	if _, err = s.Load(context.Background()); err == nil {
		t.Fatal("corrupt state accepted")
	}
	files, _ := filepath.Glob(filepath.Join(root, "status.json.corrupt-*"))
	if len(files) != 1 {
		t.Fatal("corrupt evidence lost")
	}
}
