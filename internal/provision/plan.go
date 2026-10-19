package provision

import (
	"errors"
	"fmt"
	"github.com/NikaSir/vless-gateway/internal/config"
	"net"
	"net/netip"
	"strings"
)

type Link struct {
	Interface string `json:"ifname"`
	MAC       string `json:"address"`
}
type Route struct {
	Destination string `json:"dst"`
	Device      string `json:"dev"`
}
type Inventory struct {
	Board          string   `json:"board"`
	Distribution   string   `json:"distribution"`
	Version        string   `json:"version"`
	Arch           string   `json:"arch"`
	SingBoxVersion string   `json:"sing_box_version"`
	ClashSupport   bool     `json:"clash_support"`
	Links          []Link   `json:"links"`
	Routes         []Route  `json:"routes"`
	RulePriorities []int    `json:"rule_priorities"`
	NFTTables      []string `json:"nft_tables"`
}

func Build(s config.Settings, h config.Hardware, i Inventory, owned bool) (map[string][]byte, error) {
	if e := config.ValidateTopology(s, h); e != nil {
		return nil, e
	}
	if i.Board != "friendlyarm,nanopi-r5s" || i.Distribution != "FriendlyWrt" || i.Version != "24.10.2" || i.Arch != "arm64" || i.SingBoxVersion != "1.12.22" || !i.ClashSupport {
		return nil, errors.New("unsupported hardware, OS or sing-box build")
	}
	for _, p := range h.Ports {
		found := 0
		for _, l := range i.Links {
			if l.Interface == p.Interface && strings.EqualFold(l.MAC, p.MAC) {
				found++
			}
		}
		if found != 1 {
			return nil, errors.New("physical port inventory mismatch")
		}
	}
	transit, _ := netip.ParsePrefix(s.TransitSubnet)
	for _, r := range i.Routes {
		if r.Destination == "default" || r.Destination == "0.0.0.0/0" {
			if r.Device == h.Interface(config.Management) {
				return nil, errors.New("management interface has default route")
			}
			continue
		}
		p, e := netip.ParsePrefix(r.Destination)
		if e != nil {
			if a, e := netip.ParseAddr(r.Destination); e == nil {
				p = netip.PrefixFrom(a, 32)
			}
		}
		if p.IsValid() && transit.Overlaps(p) && !(owned && r.Device == h.Interface(config.Downstream)) {
			return nil, errors.New("transit subnet overlaps live route")
		}
	}
	if !owned {
		for _, p := range i.RulePriorities {
			if p >= 9020 && p <= 9039 || p == 32760 {
				return nil, errors.New("reserved route rule already in use")
			}
		}
		for _, t := range i.NFTTables {
			if t == "inet sing-box" || t == "inet nikas_guard" {
				return nil, errors.New("reserved nft table already in use")
			}
		}
	}
	up, down, mgmt := h.Interface(config.Upstream), h.Interface(config.Downstream), h.Interface(config.Management)
	transit = transit.Masked()
	nano := transit.Addr().Next()
	mgmtNet, _ := netip.ParsePrefix(s.ManagementSubnet)
	mask := net.IP(net.CIDRMask(transit.Bits(), 32)).String()
	mgmtMask := net.IP(net.CIDRMask(mgmtNet.Bits(), 32)).String()
	_, apiPort, _ := net.SplitHostPort(s.ManagementListen)
	network := fmt.Sprintf("config interface 'loopback'\n option device 'lo'\n option proto 'static'\n option ipaddr '127.0.0.1'\n option netmask '255.0.0.0'\n\nconfig interface 'nikas_up'\n option device '%s'\n option proto 'dhcp'\n option delegate '0'\n\nconfig interface 'nikas_down'\n option device '%s'\n option proto 'static'\n option ipaddr '%s'\n option netmask '%s'\n option delegate '0'\n\nconfig interface 'nikas_mgmt'\n option device '%s'\n option proto 'static'\n option ipaddr '%s'\n option netmask '%s'\n option delegate '0'\n", up, down, nano, mask, mgmt, s.ManagementAddress, mgmtMask)
	firewall := "config defaults\n option input 'REJECT'\n option output 'ACCEPT'\n option forward 'REJECT'\n option synflood_protect '1'\n"
	for _, zone := range []string{"up", "down", "mgmt"} {
		firewall += fmt.Sprintf("\nconfig zone\n option name '%s'\n list network 'nikas_%s'\n option input 'REJECT'\n option output 'ACCEPT'\n option forward 'REJECT'\n", zone, zone)
	}
	firewall += "\nconfig forwarding\n option src 'down'\n option dest 'up'\n\nconfig rule\n option name 'Upstream DHCP'\n option src 'up'\n option proto 'udp'\n option dest_port '68'\n option family 'ipv4'\n option target 'ACCEPT'\n\nconfig rule\n option name 'Transit DNS'\n option src 'down'\n option proto 'tcp udp'\n option dest_port '53'\n option family 'ipv4'\n option target 'ACCEPT'\n"
	firewall += fmt.Sprintf("\nconfig rule\n option name 'HA read-only API'\n option src 'mgmt'\n option src_ip '%s'\n option proto 'tcp'\n option dest_port '%s'\n option family 'ipv4'\n option target 'ACCEPT'\n\nconfig rule\n option name 'Administrator SSH API'\n option src 'mgmt'\n option src_ip '%s'\n option proto 'tcp'\n option dest_port '22 %s'\n option family 'ipv4'\n option target 'ACCEPT'\n", s.HAHost, apiPort, s.AdminSubnet, apiPort)
	dns := fmt.Sprintf("config dnsmasq\n option domainneeded '1'\n option boguspriv '1'\n option noresolv '1'\n option strictorder '1'\n list interface 'nikas_down'\n list interface 'loopback'\n list server '127.0.0.1#5353'\n list server '%s'\n option localservice '1'\n\nconfig dhcp 'nikas_down'\n option interface 'nikas_down'\n option ignore '1'\n", s.DirectResolver)
	guard := fmt.Sprintf("table inet nikas_guard {\n chain forward_guard {\n type filter hook forward priority -10; policy accept;\n iifname \"%s\" reject\n oifname \"%s\" reject\n iifname \"%s\" meta nfproto ipv6 reject\n oifname \"%s\" meta nfproto ipv6 reject\n }\n chain input_guard {\n type filter hook input priority -10; policy accept;\n iifname \"%s\" ip saddr { %s, %s } tcp dport %s accept\n tcp dport %s reject\n }\n}\n", mgmt, mgmt, down, down, mgmt, s.HAHost, s.AdminSubnet, apiPort, apiPort)
	sysctl := fmt.Sprintf("net.ipv4.ip_forward=1\nnet/ipv6/conf/%s/disable_ipv6=1\nnet/ipv6/conf/%s/disable_ipv6=1\nnet/ipv6/conf/%s/forwarding=0\nnet/ipv6/conf/%s/forwarding=0\n", up, down, down, mgmt)
	return map[string][]byte{"etc/config/network": []byte(network), "etc/config/firewall": []byte(firewall), "etc/config/dhcp": []byte(dns), "usr/share/nftables.d/ruleset-post/90-nikas-vless.nft": []byte(guard), "etc/sysctl.d/90-nikas-vless.conf": []byte(sysctl)}, nil
}
