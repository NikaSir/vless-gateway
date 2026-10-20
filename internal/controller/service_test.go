package controller

import (
	"context"
	"errors"
	"github.com/NikaSir/vless-gateway/internal/clash"
	"github.com/NikaSir/vless-gateway/internal/failover"
	"github.com/NikaSir/vless-gateway/internal/model"
	"sync"
	"testing"
	"time"
)

type fakeClash struct {
	mu             sync.Mutex
	selected       string
	calls, selects int
	failSelect     bool
	failObserve    bool
	failAckOnce    bool
	pendingAck     bool
	barrier        chan struct{}
}

func (f *fakeClash) Delay(ctx context.Context, out, target string, d time.Duration) (clash.DelayResult, error) {
	f.mu.Lock()
	f.calls++
	if f.barrier != nil && f.calls%4 == 0 {
		close(f.barrier)
	}
	ch := f.barrier
	f.mu.Unlock()
	if ch != nil {
		select {
		case <-ch:
		case <-ctx.Done():
			return clash.DelayResult{}, ctx.Err()
		}
	}
	return clash.DelayResult{Latency: 20 * time.Millisecond}, nil
}
func (f *fakeClash) Selected(context.Context, string) (string, error) {
	if f.pendingAck {
		f.pendingAck = false
		return "", errors.New("ack unavailable")
	}
	if f.failObserve {
		return "", errors.New("not reachable")
	}
	return f.selected, nil
}
func (f *fakeClash) Select(_ context.Context, _, route string) error {
	f.selects++
	if f.failSelect {
		return errors.New("private error")
	}
	f.selected = route
	if f.failAckOnce {
		f.pendingAck = true
		f.failAckOnce = false
	}
	return nil
}

type clock struct{ n int64 }

func (c *clock) Now() time.Time { c.n++; return time.Unix(c.n, 0) }
func TestConcurrentCycleAndReconciliation(t *testing.T) {
	f := &fakeClash{selected: "direct", barrier: make(chan struct{})}
	m, _ := failover.New(failover.Config{BootstrapSuccesses: 2, FailureCycles: 3, RecoveryCycles: 8})
	s, err := New(Config{Interval: 15 * time.Second, Timeout: time.Second, Targets: [2]string{"https://one.example/204", "https://two.example/204"}}, f, f, m, nil, &clock{})
	if err != nil {
		t.Fatal(err)
	}
	snap, err := s.PollOnce(context.Background())
	if err != nil || f.calls != 4 || snap.ActiveRoute.Value == nil || *snap.ActiveRoute.Value != model.RouteDirect {
		t.Fatal(snap, err)
	}
	f.barrier = nil
	snap, err = s.PollOnce(context.Background())
	if err != nil || *snap.ActiveRoute.Value != model.RouteGermany || f.selects != 1 {
		t.Fatal(snap, err)
	}
	_, _ = s.PollOnce(context.Background())
	if f.selects != 1 {
		t.Fatal("redundant selector mutation")
	}
	f.failSelect = true
	if err = s.SetMode(context.Background(), model.ModePreferNetherlands); err == nil {
		t.Fatal("failed mutation not reported")
	}
	snap = s.Snapshot()
	if snap.ActiveRoute.Available || snap.State != model.ControlError {
		t.Fatal("unconfirmed route published")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before := f.selects
	_, _ = s.PollOnce(ctx)
	if f.selects != before {
		t.Fatal("shutdown changed data plane")
	}
}
func TestControllerRestartRetainsSelectionDuringBootstrap(t *testing.T) {
	f := &fakeClash{selected: "vless-de"}
	m, _ := failover.New(failover.Config{BootstrapSuccesses: 2, FailureCycles: 3, RecoveryCycles: 8})
	s, _ := New(Config{Interval: time.Second, Timeout: time.Second, Targets: [2]string{"https://one.example", "https://two.example"}}, f, f, m, nil, &clock{})
	snap, err := s.PollOnce(context.Background())
	if err != nil || f.selects != 0 || *snap.ActiveRoute.Value != model.RouteGermany {
		t.Fatal("restart interrupted traffic", snap, err)
	}
}

func TestSelectorLossInvalidatesCurrentRouteDuringFreshPolling(t *testing.T) {
	f := &fakeClash{selected: "vless-de"}
	m, _ := failover.New(failover.Config{BootstrapSuccesses: 2, FailureCycles: 3, RecoveryCycles: 8})
	s, _ := New(Config{Interval: time.Second, Timeout: time.Second, Targets: [2]string{"https://one.example", "https://two.example"}}, f, f, m, nil, &clock{})
	first, err := s.PollOnce(context.Background())
	if err != nil || !first.ActiveRoute.Available {
		t.Fatal(first, err)
	}
	f.failObserve = true
	for range 3 {
		next, err := s.PollOnce(context.Background())
		if err == nil || next.ActiveRoute.Available || next.ActiveRoute.Value != nil || next.DNSRoute.Available || !next.ObservedAt.After(first.ObservedAt) {
			t.Fatal("stale route advertised as fresh", next, err)
		}
	}
}
