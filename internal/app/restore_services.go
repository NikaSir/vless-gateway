package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

type commandRunner func(context.Context, string, ...string) error

func restartRestoredServices(ctx context.Context, root string, run commandRunner) error {
	failed := false
	attempt := func(name string, args ...string) {
		if run(ctx, name, args...) != nil {
			failed = true
		}
	}
	if _, err := os.Stat(filepath.Join(root, "etc/config/nikas-vless")); err == nil {
		attempt("uci", "set", "nikas-vless.main.policy_enabled=0")
		attempt("uci", "commit", "nikas-vless")
	}
	for _, service := range []string{"network", "firewall", "dnsmasq"} {
		attempt(filepath.Join(root, "etc/init.d", service), "restart")
	}
	if _, err := os.Stat(filepath.Join(root, "etc/init.d/nikas-gatewayd")); err == nil {
		attempt(filepath.Join(root, "etc/init.d/nikas-gatewayd"), "restart")
	}
	if failed {
		return errors.New("files restored; service recovery failed")
	}
	return nil
}
