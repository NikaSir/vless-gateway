package singbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type check struct{ fail bool }

func (c check) Check(context.Context, string) error {
	if c.fail {
		return errors.New("secret-canary")
	}
	return nil
}

type reload struct {
	fail  bool
	calls int
}

func (r *reload) Reload(context.Context) error { r.calls++; return nil }
func (r *reload) Healthy(context.Context) error {
	if r.fail {
		return errors.New("secret-canary")
	}
	return nil
}
func TestActivateRejectAndRollback(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		checkFail, healthFail bool
	}{{"validation", true, false}, {"reload", false, true}, {"success", false, false}} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config.json")
			_ = os.WriteFile(p, []byte("old"), 0600)
			r := &reload{fail: tc.healthFail}
			e := Activate(context.Background(), p, []byte("new"), check{tc.checkFail}, r)
			b, _ := os.ReadFile(p)
			if tc.checkFail || tc.healthFail {
				if e == nil || string(b) != "old" || strings.Contains(e.Error(), "canary") {
					t.Fatal("unsafe transaction", string(b), e)
				}
			} else {
				if e != nil || string(b) != "new" {
					t.Fatal(string(b), e)
				}
			}
		})
	}
}
