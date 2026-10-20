package singbox

import (
	"encoding/json"
	"github.com/NikaSir/vless-gateway/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestRenderPolicyAndIsolation(t *testing.T) {
	r := config.Runtime{Root: "/etc/nikas-vless", Settings: config.Settings{DirectResolver: "192.168.1.1", EncryptedResolver: "8.8.8.8", EncryptedServerName: "dns.google", ClashURL: "http://127.0.0.1:9090"}, Hardware: config.Hardware{Ports: []config.Port{{Role: config.Downstream, Interface: "data-down"}}}}
	b, e := Render(r)
	if e != nil {
		t.Fatal(e)
	}
	var d Document
	if e = json.Unmarshal(b, &d); e != nil {
		t.Fatal(e)
	}
	if d.Inbounds[0].InterfaceName != "nikas-tun0" || d.Inbounds[0].IncludeInterface[0] != "data-down" || d.Inbounds[0].IPRoute2TableIndex != 20220 || d.Inbounds[0].IPRoute2RuleIndex != 9020 {
		t.Fatal("TUN isolation wrong")
	}
	if d.Route.Final != "vless-active" || d.Outbounds[0].Default != "direct" {
		t.Fatal("unsafe default")
	}
	var tags []string
	for _, r := range d.Route.Rules {
		if len(r.RuleSet) > 0 {
			tags = append(tags, r.RuleSet[0])
		}
	}
	if len(tags) != 4 || tags[0] != "force-direct" || tags[1] != "force-vless" || tags[2] != "geosite-ru" || tags[3] != "geoip-ru" {
		t.Fatal("route precedence", tags)
	}
	if d.DNS.Servers[1].Detour != "vless-active" || d.DNS.Strategy != "ipv4_only" {
		t.Fatal("DNS bypasses active selector")
	}
	// JSON escaping must be structural, never executable substitution.
	r.Secrets.Germany.ServerName = "x\"\\\n"
	if _, e = Render(r); e != nil {
		t.Fatal(e)
	}
	if os.Getenv("SING_BOX_BIN") != "" {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.json")
		_ = os.WriteFile(path, b, 0600)
	}
}

func TestColdStartSelectorCannotRestorePersistentChoice(t *testing.T) {
	b, e := Render(config.Runtime{})
	if e != nil {
		t.Fatal(e)
	}
	var d Document
	if e = json.Unmarshal(b, &d); e != nil {
		t.Fatal(e)
	}
	// sing-box 1.12.22 loads stored selector choice in preference to default when cache_file is enabled.
	if d.Outbounds[0].Default != "direct" || d.Experimental.CacheFile.Enabled {
		t.Fatal("cold start may restore a stale VLESS choice", string(b))
	}
}
