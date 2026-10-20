package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/NikaSir/vless-gateway/internal/api"
	"github.com/NikaSir/vless-gateway/internal/buildinfo"
	"github.com/NikaSir/vless-gateway/internal/clash"
	"github.com/NikaSir/vless-gateway/internal/config"
	"github.com/NikaSir/vless-gateway/internal/controller"
	"github.com/NikaSir/vless-gateway/internal/failover"
	"github.com/NikaSir/vless-gateway/internal/model"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type quietClash struct{ selects int }

func (q *quietClash) Delay(context.Context, string, string, time.Duration) (clash.DelayResult, error) {
	return clash.DelayResult{Latency: time.Millisecond}, nil
}

type vanishingSelector struct {
	quietClash
	selected string
	lost     bool
}

func (q *vanishingSelector) Selected(context.Context, string) (string, error) {
	if q.lost {
		return "", errors.New("offline")
	}
	return q.selected, nil
}
func (q *vanishingSelector) Select(_ context.Context, _, route string) error {
	q.selected = route
	return nil
}

func TestPublicFreshStatusMakesLostSelectorRouteUnavailable(t *testing.T) {
	q := &vanishingSelector{selected: "direct"}
	m, _ := failover.New(failover.Config{BootstrapSuccesses: 2, FailureCycles: 3, RecoveryCycles: 8})
	c, _ := controller.New(controller.Config{Interval: time.Second, Timeout: time.Second, Targets: [2]string{"https://one.example", "https://two.example"}}, q, q, m, nil, controller.RealClock{})
	_, _ = c.PollOnce(context.Background())
	before, err := c.PollOnce(context.Background())
	if err != nil || !before.ActiveRoute.Available || *before.ActiveRoute.Value != model.RouteGermany {
		t.Fatal(before, err)
	}
	q.lost = true
	_, _ = c.PollOnce(context.Background())
	p := &GatewayProvider{Controller: c}
	h, err := api.NewServer(api.ServerConfig{ManagementListen: "192.168.10.5:9080", BearerToken: strings.Repeat("t", 32)}, p, buildinfo.Current())
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("t", 32))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var response struct {
		Freshness struct{ Available, Stale bool }
		Data      struct {
			ActiveRoute model.Metric[model.Route] `json:"active_route"`
		}
	}
	if json.Unmarshal(w.Body.Bytes(), &response) != nil || w.Code != 200 || response.Freshness.Stale || !response.Freshness.Available || response.Data.ActiveRoute.Available || response.Data.ActiveRoute.Value != nil {
		t.Fatal(w.Body.String())
	}
}

func TestProviderFallbackUsesConfiguredDirectResolver(t *testing.T) {
	p := newGatewayProvider(nil, nil, config.Runtime{Root: t.TempDir(), Settings: config.Settings{TransitSubnet: "192.168.2.0/24", DirectResolver: "192.168.1.1"}})
	if p.FallbackDNS != "192.168.1.1:53" {
		t.Fatal("fallback points at transit NanoPi rather than Keenetic", p.FallbackDNS)
	}
}
func (q *quietClash) Selected(context.Context, string) (string, error) { return "direct", nil }
func (q *quietClash) Select(context.Context, string, string) error     { q.selects++; return nil }
func TestDaemonProviderDoesNotInventInternetOrResetRoute(t *testing.T) {
	q := &quietClash{}
	m, _ := failover.New(failover.Config{BootstrapSuccesses: 2, FailureCycles: 3, RecoveryCycles: 8})
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
