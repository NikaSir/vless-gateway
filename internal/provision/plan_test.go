package provision

import (
	"github.com/NikaSir/vless-gateway/internal/config"
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T) (config.Settings, config.Hardware, Inventory) {
	t.Helper()
	var s config.Settings
	var h config.Hardware
	if config.ReadJSON("../../config/examples/settings.example.json", &s) != nil || config.ReadJSON("../../config/examples/hardware.example.json", &h) != nil {
		t.Fatal("fixtures")
	}
	links := []Link{}
	for _, p := range h.Ports {
		links = append(links, Link{Interface: p.Interface, MAC: p.MAC})
	}
	return s, h, Inventory{Board: "friendlyarm,nanopi-r5s", Distribution: "FriendlyWrt", Version: "24.10.2", Arch: "arm64", SingBoxVersion: "1.12.22", ClashSupport: true, Links: links}
}
func TestPreflightRejectsBeforeWriting(t *testing.T) {
	for _, bad := range []string{"board", "version", "mac", "route", "management-default", "reserved-rule", "reserved-table"} {
		t.Run(bad, func(t *testing.T) {
			s, h, i := fixture(t)
			switch bad {
			case "board":
				i.Board = "raspberrypi"
			case "version":
				i.Version = "other"
			case "mac":
				i.Links[2].MAC = i.Links[0].MAC
			case "route":
				i.Routes = []Route{{Destination: "192.168.2.128/25"}}
			case "management-default":
				i.Routes = []Route{{Destination: "default", Device: h.Interface(config.Management)}}
			case "reserved-rule":
				i.RulePriorities = []int{9020}
			case "reserved-table":
				i.NFTTables = []string{"inet sing-box"}
			}
			if _, e := Build(s, h, i, false); e == nil {
				t.Fatal("unsafe inventory accepted")
			}
		})
	}
}
func TestPlanNoNATAndIsolatedManagement(t *testing.T) {
	s, h, i := fixture(t)
	files, e := Build(s, h, i, false)
	if e != nil {
		t.Fatal(e)
	}
	firewall := string(files["etc/config/firewall"])
	if strings.Contains(firewall, "masq") || strings.Contains(firewall, "snat") {
		t.Fatal("NAT added")
	}
	network := string(files["etc/config/network"])
	if strings.Contains(network, "gateway") {
		t.Fatal("management default route introduced")
	}
	if !strings.Contains(network, "192.168.2.1") || !strings.Contains(network, "192.168.10.5") {
		t.Fatal("wrong addresses")
	}
	dns := string(files["etc/config/dhcp"])
	if !strings.Contains(dns, "strictorder '1'") || strings.Index(dns, "127.0.0.1#5353") > strings.Index(dns, "192.168.1.1") {
		t.Fatal("DNS fallback order")
	}
	_ = os.ErrNotExist
}
