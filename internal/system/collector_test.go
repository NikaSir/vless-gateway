package system

import (
	"errors"
	"testing"
)

func TestCollectorUnavailableIsNotZero(t *testing.T) {
	c := Collector{ReadFile: func(path string) ([]byte, error) {
		switch path {
		case "/proc/uptime":
			return []byte("123.5 7"), nil
		case "/proc/loadavg":
			return []byte("0.25 1 2 1/20 123"), nil
		default:
			return nil, errors.New("missing")
		}
	}, FreeBytes: func() (uint64, error) { return 1234, nil }}
	s := c.Collect()
	if s.Uptime.Value == nil || *s.Uptime.Value != 123.5 || s.Load.Value == nil || *s.Load.Value != 0.25 || *s.StorageFree.Value != 1234 {
		t.Fatal(s)
	}
	if s.Temperature.Available || s.Temperature.Value != nil {
		t.Fatal("missing temperature invented")
	}
}
