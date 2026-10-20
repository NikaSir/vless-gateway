package singbox

import (
	"encoding/json"
	"github.com/NikaSir/vless-gateway/internal/config"
	"net/url"
	"path/filepath"
)

func Render(r config.Runtime) ([]byte, error) {
	s := r.Settings
	root := r.Root
	if root == "" {
		root = "/etc/nikas-vless"
	}
	u, _ := url.Parse(s.ClashURL)
	addr := "127.0.0.1:9090"
	if u != nil && u.Host != "" {
		addr = u.Host
	}
	d := Document{Log: map[string]any{"level": "error", "timestamp": true},
		Inbounds:     []Inbound{{Type: "tun", Tag: "tun-in", InterfaceName: "nikas-tun0", Address: []string{"172.31.255.1/30"}, AutoRoute: true, AutoRedirect: true, IncludeInterface: []string{r.Hardware.Interface(config.Downstream)}, IPRoute2TableIndex: 20220, IPRoute2RuleIndex: 9020, FallbackIndex: 32760, InputMark: "0x2023", OutputMark: "0x2024", MTU: 1500, Stack: "system"}, {Type: "direct", Tag: "dns-in", Listen: "127.0.0.1", ListenPort: 5353}},
		Outbounds:    []Outbound{{Type: "selector", Tag: "vless-active", Outbounds: []string{"vless-de", "vless-nl", "direct"}, Default: "direct"}, {Type: "direct", Tag: "direct"}},
		DNS:          DNS{Servers: []DNSServer{{Type: "udp", Tag: "dns-direct", Server: s.DirectResolver, Detour: "direct"}, {Type: "tls", Tag: "dns-encrypted", Server: s.EncryptedResolver, ServerPort: 853, Detour: "vless-active", TLS: &TLS{Enabled: true, ServerName: s.EncryptedServerName}}}, Rules: []DNSRule{{RuleSet: []string{"force-direct"}, Action: "route", Server: "dns-direct"}, {RuleSet: []string{"force-vless"}, Action: "route", Server: "dns-encrypted"}, {RuleSet: []string{"geosite-ru"}, Action: "route", Server: "dns-direct"}}, Final: "dns-encrypted", Strategy: "ipv4_only", ReverseMapping: true},
		Experimental: Experimental{ClashAPI: ClashAPI{addr, r.Auth.ClashToken}, CacheFile: CacheFile{Enabled: false}},
		Route:        Route{AutoDetectInterface: true, Final: "vless-active"}}
	endpoints := []string{}
	for i, p := range []config.Profile{r.Secrets.Germany, r.Secrets.Netherlands} {
		tag := "vless-de"
		if i == 1 {
			tag = "vless-nl"
		}
		d.Outbounds = append(d.Outbounds, Outbound{Type: "vless", Tag: tag, Server: p.Server, ServerPort: p.ServerPort, UUID: p.UUID, Flow: "xtls-rprx-vision", TLS: &TLS{Enabled: true, ServerName: p.ServerName, UTLS: &UTLS{true, "chrome"}, Reality: &Reality{true, p.PublicKey, p.ShortID}}})
		if p.Server != "" {
			endpoints = append(endpoints, p.Server+"/32")
		}
	}
	d.Route.Rules = []Rule{{Inbound: []string{"dns-in"}, Action: "hijack-dns"}, {Action: "sniff"}, {IPIsPrivate: true, Action: "route", Outbound: "direct"}}
	if len(endpoints) > 0 {
		d.Route.Rules = append(d.Route.Rules, Rule{IPCIDR: endpoints, Action: "route", Outbound: "direct"})
	}
	for i, tag := range []string{"force-direct", "force-vless", "geosite-ru", "geoip-ru"} {
		out := "direct"
		if i == 1 {
			out = "vless-active"
		}
		d.Route.Rules = append(d.Route.Rules, Rule{RuleSet: []string{tag}, Action: "route", Outbound: out})
		d.Route.RuleSet = append(d.Route.RuleSet, RuleSet{Type: "local", Tag: tag, Format: "binary", Path: filepath.Join(root, "rules", "current", tag+".srs")})
	}
	return json.MarshalIndent(d, "", "  ")
}
