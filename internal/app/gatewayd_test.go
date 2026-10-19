package app

import (
	"context"
	"github.com/NikaSir/vless-gateway/internal/clash"
	"github.com/NikaSir/vless-gateway/internal/controller"
	"github.com/NikaSir/vless-gateway/internal/failover"
	"github.com/NikaSir/vless-gateway/internal/model"
	"testing"
	"time"
)

type quietClash struct{ selects int }

func (q *quietClash) Delay(context.Context, string, string, time.Duration) (clash.DelayResult, error) {
	return clash.DelayResult{Latency: time.Millisecond}, nil
}
func (q *quietClash) Selected(context.Context, string) (string, error) { return "direct", nil }
func (q *quietClash) Select(context.Context, string, string) error     { q.selects++; return nil }
func TestDaemonProviderDoesNotInventInternetOrResetRoute(t *testing.T) {
	q := &quietClash{}
	m, _ := failover.New(failover.Config{2, 3, 8})
	c, _ := controller.New(controller.Config{Interval: time.Second, Timeout: time.Second, Targets: [2]string{"https://one.example", "https://two.example"}}, q, q, m, nil, controller.RealClock{})
	p := &GatewayProvider{Controller: c}
	s := p.Snapshot(context.Background())
	if s.Internet.Available || s.ActiveRoute.Available {
		t.Fatal("not-yet-measured state invented")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := c.Run(ctx); e != nil {
		t.Fatal(e)
	}
	if q.selects != 0 {
		t.Fatal("shutdown reset selector")
	}
	_ = model.Starting
}
