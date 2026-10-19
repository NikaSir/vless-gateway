package config

import (
	"strings"
	"testing"
)

func TestTopologyRejectsInvalidAPIPort(t *testing.T) {
	var s Settings
	var h Hardware
	_ = ReadJSON("../../config/examples/settings.example.json", &s)
	_ = ReadJSON("../../config/examples/hardware.example.json", &h)
	for _, port := range []string{"bad", "0", "65536", "90'80"} {
		s.ManagementListen = s.ManagementAddress + ":" + port
		if ValidateTopology(s, h) == nil {
			t.Fatal("unsafe management port accepted")
		}
	}
}
func TestControlRejectsOversizedOrUntrustedProbeConfiguration(t *testing.T) {
	var s Settings
	var h Hardware
	_ = ReadJSON("../../config/examples/settings.example.json", &s)
	_ = ReadJSON("../../config/examples/hardware.example.json", &h)
	r := Runtime{Settings: s, Hardware: h, Auth: Auth{strings.Repeat("a", 40), strings.Repeat("b", 40)}}
	if ValidateControl(r) != nil {
		t.Fatal("valid fixture")
	}
	r.Settings.ClashURL = "http://192.0.2.1:9090"
	if ValidateControl(r) == nil {
		t.Fatal("Clash exposed outside loopback")
	}
}
