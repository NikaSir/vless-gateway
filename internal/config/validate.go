package config

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
)

func ValidateTopology(s Settings, h Hardware) error {
	transit, e := netip.ParsePrefix(s.TransitSubnet)
	if e != nil || !transit.Addr().Is4() || transit.Bits() > 29 {
		return errors.New("transit_subnet: invalid IPv4 network")
	}
	mgmt, e := netip.ParsePrefix(s.ManagementSubnet)
	if e != nil || !mgmt.Addr().Is4() || transit.Overlaps(mgmt) {
		return errors.New("management_subnet: invalid or overlapping")
	}
	for _, v := range s.ExistingNetworks {
		p, e := netip.ParsePrefix(v)
		if e != nil {
			return errors.New("existing_networks: invalid prefix")
		}
		if transit.Overlaps(p) {
			return errors.New("transit_subnet: overlaps existing network")
		}
	}
	ma, e := netip.ParseAddr(s.ManagementAddress)
	if e != nil || !mgmt.Contains(ma) {
		return errors.New("management_address: outside subnet")
	}
	host, port, e := net.SplitHostPort(s.ManagementListen)
	if e != nil || host != s.ManagementAddress || port == "" {
		return errors.New("management_listen: must bind management address")
	}
	ha, e := netip.ParseAddr(s.HAHost)
	if e != nil || !ha.Is4() {
		return errors.New("ha_host: invalid")
	}
	ad, e := netip.ParsePrefix(s.AdminSubnet)
	if e != nil || !ad.Addr().Is4() {
		return errors.New("admin_subnet: invalid")
	}
	roles := map[string]bool{}
	macs := map[string]bool{}
	ifs := map[string]bool{}
	for _, p := range h.Ports {
		if p.Role != Upstream && p.Role != Downstream && p.Role != Management {
			return errors.New("hardware.ports: invalid role")
		}
		mac, e := net.ParseMAC(p.MAC)
		if e != nil || len(mac) != 6 || roles[p.Role] || macs[mac.String()] || ifs[p.Interface] || !regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,15}$`).MatchString(p.Interface) || p.Label == "" {
			return errors.New("hardware.ports: ambiguous or invalid inventory")
		}
		roles[p.Role] = true
		macs[mac.String()] = true
		ifs[p.Interface] = true
	}
	if len(roles) != 3 {
		return errors.New("hardware.ports: three roles required")
	}
	return nil
}
func ValidateControl(r Runtime) error {
	if e := ValidateTopology(r.Settings, r.Hardware); e != nil {
		return e
	}
	s := r.Settings
	u, e := url.Parse(s.ClashURL)
	if e != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("clash_url: loopback HTTP required")
	}
	for _, v := range s.ProbeURLs {
		u, e := url.Parse(v)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return errors.New("probe_urls: HTTPS required")
		}
	}
	if s.ProbeURLs[0] == s.ProbeURLs[1] || s.IntervalSeconds < 1 || s.TimeoutSeconds < 1 || s.TimeoutSeconds >= s.IntervalSeconds || s.BootstrapSuccesses < 1 || s.FailureCycles < 1 || s.RecoveryCycles < s.BootstrapSuccesses {
		return errors.New("probe settings: invalid")
	}
	for _, v := range []string{s.DirectResolver, s.EncryptedResolver} {
		ip, e := netip.ParseAddr(v)
		if e != nil || !ip.Is4() {
			return errors.New("resolver: IPv4 address required")
		}
	}
	for _, v := range []string{r.Auth.APIToken, r.Auth.ClashToken} {
		if len(v) < 32 || strings.Contains(v, "REPLACE_WITH_") || strings.ContainsAny(v, "\r\n") {
			return errors.New("auth: independent random tokens required")
		}
	}
	if r.Auth.APIToken == r.Auth.ClashToken {
		return errors.New("auth: tokens must differ")
	}
	return nil
}
func Validate(r Runtime) error {
	if e := ValidateControl(r); e != nil {
		return e
	}
	for _, p := range []Profile{r.Secrets.Germany, r.Secrets.Netherlands} {
		ip, e := netip.ParseAddr(p.Server)
		if e != nil || !ip.Is4() || p.ServerPort == 0 {
			return errors.New("secrets.profile: IPv4 endpoint required")
		}
		if !regexp.MustCompile(`^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$`).MatchString(p.UUID) {
			return errors.New("secrets.profile.uuid: invalid")
		}
		key, e := base64.RawURLEncoding.DecodeString(p.PublicKey)
		if e != nil || len(key) != 32 {
			return errors.New("secrets.profile.public_key: invalid")
		}
		sid, e := hex.DecodeString(p.ShortID)
		if e != nil || len(sid) > 8 || len(sid) == 0 {
			return errors.New("secrets.profile.short_id: invalid")
		}
		if p.ServerName == "" || strings.ContainsAny(p.ServerName, "/\r\n\t ") {
			return errors.New("secrets.profile.server_name: invalid")
		}
	}
	if r.Secrets.Germany.Server == r.Secrets.Netherlands.Server {
		return errors.New("secrets.profiles: distinct endpoints required")
	}
	return nil
}
