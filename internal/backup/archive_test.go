package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupRestorePreservesSecretAndRejectsCorruption(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "etc/nikas-vless/secrets.json")
	_ = os.MkdirAll(filepath.Dir(path), 0700)
	_ = os.WriteFile(path, []byte("private-backup-canary"), 0600)
	archive, e := Create(root, filepath.Join(root, "etc/nikas-vless/backups"))
	if e != nil {
		t.Fatal(e)
	}
	info, _ := os.Stat(archive)
	if info.Mode().Perm() != 0600 {
		t.Fatal("unsafe archive permissions")
	}
	_ = os.WriteFile(path, []byte("changed"), 0600)
	if e = Restore(root, archive); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "private-backup-canary" {
		t.Fatal("backup lost secret")
	}
	_ = os.WriteFile(archive, []byte("corrupt"), 0600)
	if Restore(root, archive) == nil {
		t.Fatal("corrupt archive accepted")
	}
}
func TestBackupRefusesSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "etc/nikas-vless/rules")
	_ = os.MkdirAll(dir, 0700)
	_ = os.Symlink("../../../../outside", filepath.Join(dir, "current"))
	if _, e := Create(root, filepath.Join(root, "backups")); e == nil {
		t.Fatal("escaping symlink accepted")
	}
}
