package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/NikaSir/vless-gateway/internal/clash"
	"github.com/NikaSir/vless-gateway/internal/failover"
	"github.com/NikaSir/vless-gateway/internal/model"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type sequenceProber struct {
	cycle     int
	primary   string
	bootstrap bool
}

func (p *sequenceProber) Delay(_ context.Context, tag, _ string, _ time.Duration) (clash.DelayResult, error) {
	if (p.bootstrap && tag == p.primary && p.cycle <= 2) || (tag != p.primary && p.cycle%2 == 0) {
		return clash.DelayResult{Latency: time.Millisecond}, nil
	}
	return clash.DelayResult{}, errors.New("failed")
}
func newRegressionService(t *testing.T, p Prober, f *fakeClash, threshold int) *Service {
	t.Helper()
	m, e := failover.New(failover.Config{BootstrapSuccesses: 2, FailureCycles: threshold, RecoveryCycles: 8})
	if e != nil {
		t.Fatal(e)
	}
	s, e := New(Config{Interval: time.Second, Timeout: time.Second, Targets: [2]string{"https://one.example", "https://two.example"}}, p, f, m, nil, &clock{})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestFailOpenCannotReenterBootstrap(t *testing.T) {
	for _, tag := range []string{"vless-de", "vless-nl"} {
		for _, threshold := range []int{2, 3, 5} {
			t.Run(fmt.Sprintf("%s/failure-%d", tag, threshold), func(t *testing.T) {
				p := &sequenceProber{primary: tag, bootstrap: true}
				f := &fakeClash{selected: "direct"}
				s := newRegressionService(t, p, f, threshold)
				if tag == "vless-nl" {
					if e := s.SetMode(context.Background(), model.ModePreferNetherlands); e != nil {
						t.Fatal(e)
					}
				}
				for i := 1; i <= 12; i++ {
					p.cycle = i
					snap, e := s.PollOnce(context.Background())
					if e != nil {
						t.Fatal(e)
					}
					if i >= 2+threshold && (f.selected != "direct" || snap.State != model.Degraded) {
						t.Fatalf("cycle %d retained failed path: selected=%s snapshot=%+v", i, f.selected, snap)
					}
				}
			})
		}
	}
}
func TestRestartPreservesOnlyUntilSelectedPathFailureThreshold(t *testing.T) {
	for _, tag := range []string{"vless-de", "vless-nl"} {
		for _, threshold := range []int{2, 5} {
			t.Run(fmt.Sprintf("%s/failure-%d", tag, threshold), func(t *testing.T) {
				p := &sequenceProber{primary: tag}
				f := &fakeClash{selected: tag}
				s := newRegressionService(t, p, f, threshold)
				for i := 1; i <= threshold; i++ {
					p.cycle = i
					snap, e := s.PollOnce(context.Background())
					if e != nil {
						t.Fatal(e)
					}
					want := tag
					if i == threshold {
						want = "direct"
					}
					if f.selected != want {
						t.Fatalf("cycle %d selected=%s want=%s", i, f.selected, want)
					}
					if !snap.DNSRoute.Available || *snap.DNSRoute.Value != *snap.ActiveRoute.Value {
						t.Fatal("confirmed DNS route lost during bootstrap")
					}
				}
			})
		}
	}
}
func TestTunnelRequiresConfirmedHealthyVLESS(t *testing.T) {
	p := &sequenceProber{primary: "vless-de"}
	f := &fakeClash{selected: "direct"}
	s := newRegressionService(t, p, f, 3)
	snap, e := s.PollOnce(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if !snap.Tunnel.Available || snap.Tunnel.Value == nil || *snap.Tunnel.Value {
		t.Fatal("DIRECT must publish measured tunnel false", snap.Tunnel)
	}
	f2 := &fakeClash{selected: "direct"}
	s2 := newRegressionService(t, f2, f2, 3)
	s2.PollOnce(context.Background())
	before, _ := s2.PollOnce(context.Background())
	if before.Tunnel.Value == nil || !*before.Tunnel.Value {
		t.Fatal(before.Tunnel)
	}
	f2.failSelect = true
	if s2.SetMode(context.Background(), model.ModePreferNetherlands) == nil {
		t.Fatal("selector should fail")
	}
	if s2.Snapshot().Tunnel.Available {
		t.Fatal("failed selector advertised tunnel")
	}
}
func TestLastChangeRequiresAcknowledgementAndSurvivesRestore(t *testing.T) {
	f := &fakeClash{selected: "direct"}
	s := newRegressionService(t, f, f, 3)
	s.PollOnce(context.Background())
	before, _ := s.PollOnce(context.Background())
	f.failSelect = true
	if s.SetMode(context.Background(), model.ModePreferNetherlands) == nil {
		t.Fatal("selector should fail")
	}
	after := s.Snapshot()
	if !reflect.DeepEqual(after.LastChange, before.LastChange) {
		t.Fatalf("desired change replaced confirmed change: before=%+v after=%+v", before.LastChange, after.LastChange)
	}
	after, _ = s.PollOnce(context.Background())
	if !reflect.DeepEqual(after.LastChange, before.LastChange) {
		t.Fatal("poll replaced confirmed change")
	}
	fresh := newRegressionService(t, f, f, 3)
	fresh.Restore(before)
	restored := fresh.Snapshot()
	if !reflect.DeepEqual(restored.LastChange, before.LastChange) || !reflect.DeepEqual(restored.History, before.History) {
		t.Fatal("restore lost confirmed change")
	}
}

func TestAuthoritativeStatusContractFixtures(t *testing.T) {
	f := &fakeClash{selected: "direct"}
	s := newRegressionService(t, f, f, 3)
	s.PollOnce(context.Background())
	before, _ := s.PollOnce(context.Background())
	f.failSelect = true
	s.SetMode(context.Background(), model.ModePreferNetherlands)
	after := s.Snapshot()
	p := &sequenceProber{primary: "vless-de"}
	direct := newRegressionService(t, p, &fakeClash{selected: "direct"}, 3)
	for i := 1; i <= 3; i++ {
		p.cycle = i
		direct.PollOnce(context.Background())
	}
	for name, snap := range map[string]model.StatusSnapshot{"status-confirmed-change": before, "status-selector-unconfirmed": after, "status-direct-tunnel-failed": direct.Snapshot()} {
		b, _ := json.Marshal(snap)
		path := filepath.Join("../../tests/contract/api-v1", name+".json")
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Errorf("fixture %s missing: %s", name, b)
			continue
		}
		var fixture struct {
			Data json.RawMessage `json:"data"`
		}
		if e = json.Unmarshal(raw, &fixture); e != nil {
			t.Fatal(e)
		}
		var got, want any
		json.Unmarshal(b, &got)
		json.Unmarshal(fixture.Data, &want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("fixture %s differs from controller: %s", name, b)
		}
	}
}

func TestDelayedAcknowledgementRecordsConfirmedChangeOnce(t *testing.T) {
	f := &fakeClash{selected: "direct"}
	s := newRegressionService(t, f, f, 3)
	s.PollOnce(context.Background())
	before, _ := s.PollOnce(context.Background())
	f.failAckOnce = true
	if s.SetMode(context.Background(), model.ModePreferNetherlands) == nil {
		t.Fatal("ack should be missing")
	}
	missing := s.Snapshot()
	if !reflect.DeepEqual(missing.LastChange, before.LastChange) || len(missing.History) != len(before.History) {
		t.Fatal("change published before acknowledgement")
	}
	confirmed, e := s.PollOnce(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if confirmed.LastChange == nil || confirmed.LastChange.Current != model.RouteNetherlands || confirmed.LastChange.Previous != model.RouteGermany || confirmed.LastChange.Trigger != "selector_confirmed" || !confirmed.LastChange.At.After(before.LastChange.At) || len(confirmed.History) != len(before.History)+1 {
		t.Fatal("delayed acknowledgement not recorded", confirmed.LastChange, confirmed.History)
	}
	again, e := s.PollOnce(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(again.LastChange, confirmed.LastChange) || len(again.History) != len(confirmed.History) {
		t.Fatal("duplicate confirmed change")
	}
}
