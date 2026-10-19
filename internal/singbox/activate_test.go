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

type interruptedReload struct{ duringHealth bool }

func (r interruptedReload) Reload(context.Context) error {
	if !r.duringHealth {
		panic("simulated power loss after publication")
	}
	return nil
}
func (r interruptedReload) Healthy(context.Context) error {
	panic("simulated power loss before acknowledgement")
}

func TestRetryRecoversUnconfirmedConfigBeforeReplacingKnownGood(t *testing.T) {
	for _, duringHealth := range []bool{false, true} {
		t.Run(map[bool]string{false: "after-publication", true: "before-health-ack"}[duringHealth], func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config.json")
			_ = os.WriteFile(p, []byte("known-good"), 0600)
			func() {
				defer func() {
					if recover() == nil {
						t.Fatal("interruption not simulated")
					}
				}()
				_ = Activate(context.Background(), p, []byte("unconfirmed"), check{}, interruptedReload{duringHealth})
			}()
			r := &reload{fail: true}
			if Activate(context.Background(), p, []byte("retry"), check{}, r) == nil {
				t.Fatal("unhealthy activation accepted")
			}
			b, _ := os.ReadFile(p)
			previous, _ := os.ReadFile(p + ".previous")
			if string(b) != "known-good" || string(previous) != "known-good" {
				t.Fatal("last known good overwritten", string(b), string(previous))
			}
		})
	}
}

func TestStartupRecoveryIsIdempotentAndRejectsCorruptJournal(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	_ = os.WriteFile(p, []byte("known-good"), 0600)
	func() {
		defer func() { _ = recover() }()
		_ = Activate(context.Background(), p, []byte("unconfirmed"), check{}, interruptedReload{})
	}()
	if recovered, err := Recover(p); err != nil || !recovered {
		t.Fatal(recovered, err)
	}
	if recovered, err := Recover(p); err != nil || recovered {
		t.Fatal("recovery not idempotent", recovered, err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "known-good" {
		t.Fatal(string(b))
	}
	_ = os.WriteFile(p+".transaction", []byte("null"), 0600)
	if _, err := Recover(p); err == nil {
		t.Fatal("corrupt marker accepted")
	}
	b, _ = os.ReadFile(p)
	if string(b) != "known-good" {
		t.Fatal("corrupt journal changed target")
	}
}
