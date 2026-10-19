package failover

import (
	"github.com/NikaSir/vless-gateway/internal/model"
	"testing"
	"time"
)

func TestMachineOrderedFailoverAndRecovery(t *testing.T) {
	m, err := New(Config{2, 3, 8})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	step := func(de, nl bool, want model.Route) {
		t.Helper()
		n++
		p := func(ok bool) model.RouteProbe {
			return model.RouteProbe{Targets: [2]model.ProbeTargetResult{{Success: ok, Latency: 30 * time.Millisecond}, {Success: false}}}
		}
		d := m.Observe(model.ProbeCycle{At: time.Unix(int64(n), 0), Germany: p(de), Netherlands: p(nl), Complete: true})
		if !d.Accepted || d.Snapshot.DesiredRoute != want {
			t.Fatalf("cycle %d: %+v want %s", n, d, want)
		}
	}
	step(true, true, model.RouteDirect)
	step(true, true, model.RouteGermany)
	step(false, true, model.RouteGermany)
	step(false, true, model.RouteGermany)
	step(false, true, model.RouteNetherlands)
	step(false, false, model.RouteNetherlands)
	step(false, false, model.RouteNetherlands)
	step(false, false, model.RouteDirect)
	if m.Snapshot().State != model.Degraded {
		t.Fatal("fail-open not reported")
	}
	for i := 0; i < 7; i++ {
		step(true, true, model.RouteDirect)
	}
	step(true, true, model.RouteGermany)
	s := m.Snapshot()
	if s.Germany.LatencyMS.Value == nil || *s.Germany.LatencyMS.Value != 30 {
		t.Fatal("missing true latency")
	}
	if s.LastChange == nil || s.LastChange.Previous != model.RouteDirect || s.LastChange.Current != model.RouteGermany || s.LastChange.At.Location() != time.UTC {
		t.Fatal("route-change evidence lost")
	}
	// Mutation of an exported snapshot must not modify the machine.
	s.LastChange.Current = model.RouteDirect
	if m.Snapshot().LastChange.Current != model.RouteGermany {
		t.Fatal("mutable snapshot aliases state")
	}
}
func TestMachineRejectsStaleIncompleteAndInvalidModes(t *testing.T) {
	m, _ := New(Config{2, 3, 8})
	c := model.ProbeCycle{At: time.Unix(10, 0), Complete: true}
	if !m.Observe(c).Accepted {
		t.Fatal("valid cycle rejected")
	}
	if m.Observe(c).Accepted {
		t.Fatal("duplicate accepted")
	}
	c.At = time.Unix(9, 0)
	if m.Observe(c).Accepted {
		t.Fatal("older accepted")
	}
	c.At = time.Unix(11, 0)
	c.Complete = false
	if m.Observe(c).Accepted {
		t.Fatal("incomplete accepted")
	}
	if m.SetMode("blackhole") == nil {
		t.Fatal("invalid mode accepted")
	}
	for _, mode := range []model.Mode{model.ModeAuto, model.ModePreferGermany, model.ModePreferNetherlands, model.ModeDirect} {
		if m.SetMode(mode) != nil {
			t.Fatal(mode)
		}
	}
	if _, err := New(Config{0, 3, 8}); err == nil {
		t.Fatal("zero threshold accepted")
	}
}
func TestMachinePreferredNetherlandsAndCounterReset(t *testing.T) {
	m, _ := New(Config{2, 3, 8})
	_ = m.SetMode(model.ModePreferNetherlands)
	for i, success := range []bool{true, false, true, true} {
		p := model.RouteProbe{Targets: [2]model.ProbeTargetResult{{Success: success, Latency: 5 * time.Millisecond}, {Success: success, Latency: 9 * time.Millisecond}}}
		d := m.Observe(model.ProbeCycle{At: time.Unix(int64(i+1), 0), Germany: p, Netherlands: p, Complete: true})
		want := model.RouteDirect
		if i == 3 {
			want = model.RouteNetherlands
		}
		if d.Snapshot.DesiredRoute != want {
			t.Fatalf("cycle %d got %s", i, d.Snapshot.DesiredRoute)
		}
	}
	_ = m.SetMode(model.ModeDirect)
	if m.Snapshot().DesiredRoute != model.RouteDirect {
		t.Fatal("explicit direct ignored")
	}
}
