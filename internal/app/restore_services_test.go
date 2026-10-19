package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRestoredServiceFailuresAreReportedAfterAttemptingAllRecovery(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"etc/config/nikas-vless", "etc/init.d/nikas-gatewayd"} {
		full := filepath.Join(root, p)
		_ = os.MkdirAll(filepath.Dir(full), 0700)
		_ = os.WriteFile(full, []byte("fixture"), 0600)
	}
	for fail := 0; fail < 6; fail++ {
		calls := 0
		err := restartRestoredServices(context.Background(), root, func(context.Context, string, ...string) error {
			calls++
			if calls == fail+1 {
				return errors.New("failed")
			}
			return nil
		})
		if err == nil || calls != 6 {
			t.Fatalf("failure %d hidden or recovery stopped early: calls=%d, err=%v", fail, calls, err)
		}
	}
}
