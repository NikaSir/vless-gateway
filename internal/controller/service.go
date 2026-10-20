package controller

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/NikaSir/vless-gateway/internal/clash"
	"github.com/NikaSir/vless-gateway/internal/failover"
	"github.com/NikaSir/vless-gateway/internal/model"
	"sync"
	"time"
)

type Config struct {
	Interval, Timeout time.Duration
	Targets           [2]string
}
type Prober interface {
	Delay(context.Context, string, string, time.Duration) (clash.DelayResult, error)
}
type Selector interface {
	Selected(context.Context, string) (string, error)
	Select(context.Context, string, string) error
}
type SnapshotSink interface {
	Save(context.Context, model.StatusSnapshot) error
}
type Clock interface{ Now() time.Time }
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }

type Service struct {
	mu                sync.Mutex
	cfg               Config
	p                 Prober
	sel               Selector
	m                 *failover.Machine
	sink              SnapshotSink
	clock             Clock
	s                 model.StatusSnapshot
	bootstrapComplete bool
	pendingChange     *model.RouteChange
}

func New(c Config, p Prober, sel Selector, m *failover.Machine, sink SnapshotSink, clock Clock) (*Service, error) {
	if c.Interval <= 0 || c.Timeout <= 0 || c.Targets[0] == "" || c.Targets[1] == "" || p == nil || sel == nil || m == nil || clock == nil {
		return nil, errors.New("invalid controller configuration")
	}
	return &Service{cfg: c, p: p, sel: sel, m: m, sink: sink, clock: clock, s: model.StatusSnapshot{FailoverSnapshot: m.Snapshot(), ActiveRoute: model.Unavailable[model.Route]("not_observed"), DNSRoute: model.Unavailable[model.Route]("not_observed"), Internet: model.Unavailable[bool]("not_measured"), LAN: model.Unavailable[bool]("not_measured"), Tunnel: model.Unavailable[bool]("not_observed"), BypassCount: model.Unavailable[int]("not_measured")}}, nil
}
func routeTag(r model.Route) string {
	switch r {
	case model.RouteGermany:
		return "vless-de"
	case model.RouteNetherlands:
		return "vless-nl"
	default:
		return "direct"
	}
}
func parseTag(s string) (model.Route, bool) {
	switch s {
	case "vless-de":
		return model.RouteGermany, true
	case "vless-nl":
		return model.RouteNetherlands, true
	case "direct":
		return model.RouteDirect, true
	}
	return "", false
}
func (s *Service) PollOnce(ctx context.Context) (model.StatusSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil {
		return s.clone(), ctx.Err()
	}
	cycle := model.ProbeCycle{At: s.clock.Now().UTC(), Complete: true}
	var wg sync.WaitGroup
	for i, tag := range []string{"vless-de", "vless-nl"} {
		for j, target := range s.cfg.Targets {
			wg.Add(1)
			go func(i, j int, tag, target string) {
				defer wg.Done()
				pc, cancel := context.WithTimeout(ctx, s.cfg.Timeout+time.Second)
				defer cancel()
				d, e := s.p.Delay(pc, tag, target, s.cfg.Timeout)
				r := model.ProbeTargetResult{Success: e == nil, Latency: d.Latency}
				if i == 0 {
					cycle.Germany.Targets[j] = r
				} else {
					cycle.Netherlands.Targets[j] = r
				}
			}(i, j, tag, target)
		}
	}
	wg.Wait()
	if ctx.Err() != nil {
		return s.clone(), ctx.Err()
	}
	d := s.m.Observe(cycle)
	if !d.Accepted {
		return s.clone(), errors.New(d.Error)
	}
	s.setDecision(d.Snapshot)
	err := s.reconcile(ctx)
	if s.sink != nil {
		if e := s.sink.Save(ctx, s.s); e != nil && err == nil {
			err = errors.New("state persistence failed")
		}
	}
	return s.clone(), err
}

// Machine LastChange describes intent; the public snapshot retains only acknowledged changes.
func (s *Service) setDecision(v model.FailoverSnapshot) {
	confirmed := s.s.LastChange
	s.s.FailoverSnapshot = v
	s.s.LastChange = confirmed
}
func (s *Service) updateTunnel() {
	if !s.s.ActiveRoute.Available || s.s.ActiveRoute.Value == nil {
		s.s.Tunnel = model.Unavailable[bool]("selector_unconfirmed")
		return
	}
	switch *s.s.ActiveRoute.Value {
	case model.RouteDirect:
		s.s.Tunnel = model.Available(false)
	case model.RouteGermany:
		if s.s.Germany.Available {
			s.s.Tunnel = model.Available(s.s.Germany.Healthy)
		} else {
			s.s.Tunnel = model.Unavailable[bool]("not_measured")
		}
	case model.RouteNetherlands:
		if s.s.Netherlands.Available {
			s.s.Tunnel = model.Available(s.s.Netherlands.Healthy)
		} else {
			s.s.Tunnel = model.Unavailable[bool]("not_measured")
		}
	}
}
func (s *Service) reconcile(ctx context.Context) error {
	defer s.updateTunnel()
	selected, e := s.sel.Selected(ctx, "vless-active")
	if e != nil {
		s.s.ActiveRoute = model.Unavailable[model.Route]("selector_unavailable")
		s.s.DNSRoute = model.Unavailable[model.Route]("selector_unavailable")
		s.s.State = model.ControlError
		return errors.New("selector unavailable")
	}
	actual, ok := parseTag(selected)
	if !ok {
		s.s.ActiveRoute = model.Unavailable[model.Route]("unknown_selector")
		s.s.DNSRoute = model.Unavailable[model.Route]("unknown_selector")
		s.s.State = model.ControlError
		return errors.New("invalid selector state")
	}
	s.s.ActiveRoute = model.Available(actual)
	s.s.DNSRoute = s.s.ActiveRoute
	if s.pendingChange != nil && s.pendingChange.Current == actual {
		s.confirmPendingChange()
	}
	desired := s.s.DesiredRoute
	// Restarting the controller alone does not reset an already selected route.
	if !s.bootstrapComplete {
		s.bootstrapComplete = actual == model.RouteDirect || s.m.BootstrapReady(actual)
		if !s.bootstrapComplete {
			s.s.State = model.Starting
			return nil
		}
	}
	if desired != actual {
		if s.sel.Select(ctx, "vless-active", routeTag(desired)) != nil {
			s.s.ActiveRoute = model.Unavailable[model.Route]("selector_unconfirmed")
			s.s.DNSRoute = model.Unavailable[model.Route]("selector_unconfirmed")
			s.s.State = model.ControlError
			return errors.New("selector change failed")
		}
		s.pendingChange = &model.RouteChange{Previous: actual, Current: desired, Trigger: "selector_confirmed"}
		if decision := s.m.Snapshot().LastChange; decision != nil {
			s.pendingChange.Probes = decision.Probes
		}
		confirmed, e := s.sel.Selected(ctx, "vless-active")
		if e != nil || confirmed != routeTag(desired) {
			s.s.ActiveRoute = model.Unavailable[model.Route]("selector_unconfirmed")
			s.s.DNSRoute = model.Unavailable[model.Route]("selector_unconfirmed")
			s.s.State = model.ControlError
			return errors.New("selector acknowledgement missing")
		}
		s.confirmPendingChange()
		s.s.ActiveRoute = model.Available(desired)
	}
	s.s.DNSRoute = s.s.ActiveRoute
	return nil
}
func (s *Service) confirmPendingChange() {
	change := *s.pendingChange
	// This is observation time, including a delayed acknowledgement, not an inferred switch time.
	change.At = s.clock.Now().UTC()
	s.s.LastChange = &change
	s.s.History = append(s.s.History, change)
	if len(s.s.History) > 128 {
		s.s.History = s.s.History[len(s.s.History)-128:]
	}
	s.pendingChange = nil
}
func (s *Service) SetMode(ctx context.Context, mode model.Mode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.m.SetMode(mode); err != nil {
		return err
	}
	s.setDecision(s.m.Snapshot())
	e := s.reconcile(ctx)
	if e == nil && s.sink != nil {
		e = s.sink.Save(ctx, s.s)
	}
	return e
}
func (s *Service) Restore(v model.StatusSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.m.SetMode(v.Mode)
	s.setDecision(s.m.Snapshot())
	s.s.History = append([]model.RouteChange(nil), v.History...)
	s.s.LastChange = nil
	if v.LastChange != nil && v.LastChange.Trigger == "selector_confirmed" {
		change := *v.LastChange
		s.s.LastChange = &change
	} else if n := len(s.s.History); n > 0 && s.s.History[n-1].Trigger == "selector_confirmed" {
		change := s.s.History[n-1]
		s.s.LastChange = &change
	}
}
func (s *Service) clone() model.StatusSnapshot {
	var v model.StatusSnapshot
	b, _ := json.Marshal(s.s)
	_ = json.Unmarshal(b, &v)
	return v
}
func (s *Service) Snapshot() model.StatusSnapshot { s.mu.Lock(); defer s.mu.Unlock(); return s.clone() }
func (s *Service) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()
	for {
		_, _ = s.PollOnce(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
