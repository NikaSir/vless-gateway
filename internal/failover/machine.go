package failover

import (
	"errors"
	"github.com/NikaSir/vless-gateway/internal/model"
	"time"
)

type Config struct{ BootstrapSuccesses, FailureCycles, RecoveryCycles int }
type path struct {
	health     model.RouteHealth
	recovering bool
}
type Machine struct {
	cfg       Config
	s         model.FailoverSnapshot
	de, nl    path
	lastCycle model.ProbeCycle
}

func New(c Config) (*Machine, error) {
	if c.BootstrapSuccesses < 1 || c.FailureCycles < 1 || c.RecoveryCycles < c.BootstrapSuccesses {
		return nil, errors.New("invalid failover thresholds")
	}
	return &Machine{cfg: c, s: model.FailoverSnapshot{Mode: model.ModeAuto, DesiredRoute: model.RouteDirect, State: model.Starting}}, nil
}
func (m *Machine) SetMode(mode model.Mode) error {
	switch mode {
	case model.ModeAuto, model.ModePreferGermany, model.ModePreferNetherlands, model.ModeDirect:
	default:
		return errors.New("invalid route mode")
	}
	m.s.Mode = mode
	m.choose("mode_changed")
	return nil
}
func (m *Machine) update(p *path, r model.RouteProbe) {
	success := false
	latency := time.Duration(0)
	for _, v := range r.Targets {
		if v.Success {
			if !success || v.Latency < latency {
				latency = v.Latency
			}
			success = true
		}
	}
	p.health.Available = true
	if success {
		p.health.Successes++
		p.health.Failures = 0
		p.health.LatencyMS = model.Available(float64(latency) / float64(time.Millisecond))
		threshold := m.cfg.BootstrapSuccesses
		if p.recovering {
			threshold = m.cfg.RecoveryCycles
		}
		if p.health.Successes >= threshold {
			p.health.Healthy = true
		}
	} else {
		p.health.Failures++
		p.health.Successes = 0
		p.health.LatencyMS = model.Unavailable[float64]("probe_failed")
		if p.health.Failures >= m.cfg.FailureCycles {
			p.health.Healthy = false
			p.recovering = true
		}
	}
}
func (m *Machine) Observe(c model.ProbeCycle) model.Decision {
	if !c.Complete || c.At.IsZero() {
		return model.Decision{Error: "incomplete_cycle", Snapshot: m.Snapshot()}
	}
	if !c.At.After(m.s.ObservedAt) {
		return model.Decision{Error: "stale_cycle", Snapshot: m.Snapshot()}
	}
	c.At = c.At.UTC()
	m.lastCycle = c
	m.s.ObservedAt = c.At
	m.update(&m.de, c.Germany)
	m.update(&m.nl, c.Netherlands)
	m.choose("probe_health")
	return model.Decision{Accepted: true, Snapshot: m.Snapshot()}
}
func (m *Machine) choose(trigger string) {
	next := model.RouteDirect
	if m.s.Mode != model.ModeDirect {
		if m.s.Mode == model.ModePreferNetherlands {
			if m.nl.health.Healthy {
				next = model.RouteNetherlands
			} else if m.de.health.Healthy {
				next = model.RouteGermany
			}
		} else {
			if m.de.health.Healthy {
				next = model.RouteGermany
			} else if m.nl.health.Healthy {
				next = model.RouteNetherlands
			}
		}
	}
	if next != m.s.DesiredRoute {
		m.s.LastChange = &model.RouteChange{At: m.s.ObservedAt, Previous: m.s.DesiredRoute, Current: next, Trigger: trigger, Probes: m.lastCycle}
		m.s.DesiredRoute = next
	}
	m.s.State = model.Healthy
	if next == model.RouteDirect && m.s.Mode != model.ModeDirect {
		m.s.State = model.Degraded
		if m.s.ObservedAt.IsZero() {
			m.s.State = model.Starting
		}
	}
}
func (m *Machine) Snapshot() model.FailoverSnapshot {
	s := m.s
	s.Germany = m.de.health
	s.Netherlands = m.nl.health
	if s.LastChange != nil {
		c := *s.LastChange
		s.LastChange = &c
	}
	if s.Germany.LatencyMS.Value != nil {
		v := *s.Germany.LatencyMS.Value
		s.Germany.LatencyMS.Value = &v
	}
	if s.Netherlands.LatencyMS.Value != nil {
		v := *s.Netherlands.LatencyMS.Value
		s.Netherlands.LatencyMS.Value = &v
	}
	return s
}

// BootstrapReady permits reconciliation once probes qualify a route or disqualify
// the route already selected by a running sing-box. The controller latches this
// readiness so later failures cannot re-enter startup preservation.
func (m *Machine) BootstrapReady(selected model.Route) bool {
	if m.s.Mode == model.ModeDirect || m.de.health.Healthy || m.nl.health.Healthy {
		return true
	}
	switch selected {
	case model.RouteGermany:
		return m.de.health.Failures >= m.cfg.FailureCycles
	case model.RouteNetherlands:
		return m.nl.health.Failures >= m.cfg.FailureCycles
	}
	return false
}
