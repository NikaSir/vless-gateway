package singbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type configRuntime struct {
	target, active                 string
	reloads, checks                int
	reloadFailures, healthFailures map[int]bool
	loaded                         []string
}

func (r *configRuntime) Reload(context.Context) error {
	r.reloads++
	if r.reloadFailures[r.reloads] {
		return errors.New("private restart error")
	}
	b, err := os.ReadFile(r.target)
	if err != nil {
		return err
	}
	r.active = string(b)
	r.loaded = append(r.loaded, r.active)
	return nil
}
func (r *configRuntime) Healthy(context.Context) error {
	r.checks++
	if r.healthFailures[r.checks] {
		return errors.New("private health error")
	}
	return nil
}
func assertKnownGoodBytes(t *testing.T, target string) {
	t.Helper()
	for _, p := range []string{target, target + ".previous"} {
		b, err := os.ReadFile(p)
		if err != nil || string(b) != "known-good" {
			t.Fatalf("known good lost: %s=%q %v", p, b, err)
		}
	}
}
func assertConfigJournal(t *testing.T, target string, want bool) {
	t.Helper()
	_, err := os.Stat(target + ".transaction")
	if want && err != nil || !want && !os.IsNotExist(err) {
		t.Fatalf("journal presence want %t: %v", want, err)
	}
}
func TestConfigRollbackRequiresReloadAndHealthAcknowledgement(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		reloadFailures, healthFailures map[int]bool
		wantError                      string
		journal                        bool
		wantChecks                     int
	}{
		{"candidate-and-rollback-restart-fail", map[int]bool{1: true, 2: true}, nil, "rollback reload failed", true, 0},
		{"candidate-unhealthy-rollback-restart-fails", map[int]bool{2: true}, map[int]bool{1: true}, "rollback reload failed", true, 1},
		{"rollback-health-fails", nil, map[int]bool{1: true, 2: true}, "rollback health not confirmed", true, 2},
		{"candidate-health-fails-rollback-acknowledged", nil, map[int]bool{1: true}, "previous generation restored", false, 2},
		{"candidate-restart-fails-rollback-acknowledged", map[int]bool{1: true}, nil, "previous generation restored", false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(target, []byte("known-good"), 0600); err != nil {
				t.Fatal(err)
			}
			r := &configRuntime{target: target, active: "known-good", reloadFailures: tc.reloadFailures, healthFailures: tc.healthFailures}
			err := Activate(context.Background(), target, []byte("candidate"), check{}, r)
			if err == nil || !strings.Contains(err.Error(), tc.wantError) || strings.Contains(err.Error(), "private") {
				t.Fatalf("factual rollback result missing: %v (runtime=%q)", err, r.active)
			}
			assertKnownGoodBytes(t, target)
			assertConfigJournal(t, target, tc.journal)
			if r.checks != tc.wantChecks {
				t.Fatalf("health checks=%d want=%d", r.checks, tc.wantChecks)
			}
			if !tc.journal && r.active != "known-good" {
				t.Fatal("restoration claimed for candidate runtime")
			}
		})
	}
}
func TestConfigRecoveryRetryKeepsJournalUntilRuntimeAcknowledged(t *testing.T) {
	target := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(target, []byte("known-good"), 0600)
	first := &configRuntime{target: target, active: "known-good", reloadFailures: map[int]bool{2: true}, healthFailures: map[int]bool{1: true}}
	if err := Activate(context.Background(), target, []byte("candidate"), check{}, first); err == nil {
		t.Fatal("candidate accepted")
	}
	for _, r := range []*configRuntime{
		{target: target, active: "candidate", reloadFailures: map[int]bool{1: true}},
		{target: target, active: "candidate", healthFailures: map[int]bool{1: true}},
	} {
		if err := Activate(context.Background(), target, []byte("retry"), check{}, r); err == nil {
			t.Fatal("recovery accepted without runtime acknowledgement")
		}
		assertKnownGoodBytes(t, target)
		assertConfigJournal(t, target, true)
		for _, loaded := range r.loaded {
			if loaded == "retry" {
				t.Fatal("candidate staged before recovery acknowledgement")
			}
		}
	}
	success := &configRuntime{target: target, active: "candidate"}
	if err := Activate(context.Background(), target, []byte("retry"), check{}, success); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(success.loaded, []string{"known-good", "retry"}) {
		t.Fatal("recovery did not precede new candidate", success.loaded)
	}
	previous, _ := os.ReadFile(target + ".previous")
	if string(previous) != "known-good" {
		t.Fatal("retry replaced known-good backup", string(previous))
	}
	assertConfigJournal(t, target, false)
}
func TestConfigDiskOnlyRecoveryClearsAfterRestore(t *testing.T) {
	target := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(target, []byte("known-good"), 0600)
	r := &configRuntime{target: target, active: "known-good", reloadFailures: map[int]bool{2: true}, healthFailures: map[int]bool{1: true}}
	Activate(context.Background(), target, []byte("candidate"), check{}, r)
	recovered, err := Recover(target)
	if err != nil || !recovered {
		t.Fatal("disk startup did not recover", recovered, err)
	}
	assertKnownGoodBytes(t, target)
	assertConfigJournal(t, target, false)
	if r.active != "candidate" {
		t.Fatal("disk-only recovery altered runtime")
	}
	if recovered, err = Recover(target); err != nil || recovered {
		t.Fatal("disk-only recovery not idempotent", recovered, err)
	}
}

func TestConfigDiskOnlyRecoveryWithoutPreviousConfig(t *testing.T) {
	target := filepath.Join(t.TempDir(), "config.json")
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("publication not interrupted")
			}
		}()
		Activate(context.Background(), target, []byte("candidate"), check{}, interruptedReload{})
	}()
	recovered, err := Recover(target)
	if err != nil || !recovered {
		t.Fatal("first-install recovery failed", recovered, err)
	}
	if _, err = os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("unconfirmed first config remains", err)
	}
	assertConfigJournal(t, target, false)
}
