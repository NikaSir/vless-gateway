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

func TestRestoreReconcilesFirstInstallAndUpgradeWithoutRemovingBackups(t *testing.T) {
	for _, upgrade := range []bool{false, true} {
		t.Run(map[bool]string{false: "first-install", true: "upgrade"}[upgrade], func(t *testing.T) {
			root := t.TempDir()
			write := func(name, value string) {
				t.Helper()
				p := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			write("etc/config/network", "original-network")
			write("etc/unrelated", "keep")
			if upgrade {
				write("etc/nikas-vless/settings.json", "original-settings")
				_ = os.MkdirAll(filepath.Join(root, "etc/rc.d"), 0700)
				if err := os.Symlink("../init.d/nikas-gatewayd", filepath.Join(root, "etc/rc.d/S96nikas-gatewayd")); err != nil {
					t.Fatal(err)
				}
			}
			archive, err := Create(root, filepath.Join(root, "etc/nikas-vless/backups"))
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range []string{"etc/config/network", "etc/nikas-vless/settings.json", "etc/nikas-vless/new-file", "usr/share/nftables.d/ruleset-post/90-nikas-vless.nft", "etc/sysctl.d/90-nikas-vless.conf", "etc/init.d/nikas-sing-box", "usr/bin/gatewayd"} {
				write(p, "installed")
			}
			_ = os.MkdirAll(filepath.Join(root, "etc/rc.d"), 0700)
			_ = os.Symlink("../init.d/nikas-sing-box", filepath.Join(root, "etc/rc.d/S95nikas-sing-box"))
			if err = Restore(root, archive); err != nil {
				t.Fatal(err)
			}
			for _, p := range []string{"etc/nikas-vless/new-file", "usr/share/nftables.d/ruleset-post/90-nikas-vless.nft", "etc/sysctl.d/90-nikas-vless.conf", "etc/init.d/nikas-sing-box", "usr/bin/gatewayd", "etc/rc.d/S95nikas-sing-box"} {
				if _, err = os.Lstat(filepath.Join(root, p)); !os.IsNotExist(err) {
					t.Errorf("new path survived restore: %s", p)
				}
			}
			for _, p := range []string{archive, archive + ".sha256", filepath.Join(root, "etc/unrelated")} {
				if _, err = os.Stat(p); err != nil {
					t.Fatal("unrelated or backup removed", p, err)
				}
			}
			if upgrade {
				b, _ := os.ReadFile(filepath.Join(root, "etc/nikas-vless/settings.json"))
				if string(b) != "original-settings" {
					t.Fatal("upgrade state lost")
				}
				if _, err = os.Readlink(filepath.Join(root, "etc/rc.d/S96nikas-gatewayd")); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestRestoreRejectsDestinationSymlinkBeforeChangingFiles(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "etc/config"), 0700)
	_ = os.WriteFile(filepath.Join(root, "etc/config/network"), []byte("original"), 0600)
	archive, err := Create(root, filepath.Join(root, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	_ = os.RemoveAll(filepath.Join(root, "etc/config"))
	_ = os.Symlink(outside, filepath.Join(root, "etc/config"))
	if Restore(root, archive) == nil {
		t.Fatal("symlink destination accepted")
	}
	if _, err = os.Stat(filepath.Join(outside, "network")); !os.IsNotExist(err) {
		t.Fatal("restore escaped root")
	}
}
