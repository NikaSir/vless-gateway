package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigLoadAndValidation(t *testing.T) {
	root := t.TempDir()
	for name, data := range map[string]string{
		"settings.json": `{"transit_subnet":"192.168.2.0/24","management_address":"192.168.10.5","management_subnet":"192.168.10.0/24","management_listen":"192.168.10.5:9080","ha_host":"192.168.10.10","admin_subnet":"192.168.10.0/24","existing_networks":["192.168.1.0/24"],"direct_resolver":"192.168.1.1","encrypted_resolver":"8.8.8.8","encrypted_server_name":"dns.google","clash_url":"http://127.0.0.1:9090","probe_urls":["https://one.example/204","https://two.example/204"]}`,
		"hardware.json": `{"ports":[{"role":"upstream_2_5g","interface":"eth1","mac":"02:00:00:00:00:01","label":"UP"},{"role":"downstream_2_5g","interface":"eth2","mac":"02:00:00:00:00:02","label":"DOWN"},{"role":"management_1g","interface":"eth0","mac":"02:00:00:00:00:03","label":"MGMT"}]}`,
		"auth.json":     `{"api_token":"REPLACE_WITH_API","clash_token":"REPLACE_WITH_CLASH"}`,
		"secrets.json":  `{"germany":{"server":"192.0.2.1","server_port":443,"uuid":"REPLACE_WITH_UUID","server_name":"one.example","public_key":"REPLACE_WITH_KEY","short_id":"REPLACE_WITH_ID"},"netherlands":{"server":"192.0.2.2","server_port":443,"uuid":"REPLACE_WITH_UUID","server_name":"two.example","public_key":"REPLACE_WITH_KEY","short_id":"REPLACE_WITH_ID"}}`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Parsing examples is allowed for offline generation; live validation rejects placeholders.
	r, e := Load(root)
	if e != nil {
		t.Fatal(e)
	}
	if r.Settings.BootstrapSuccesses != 2 || r.Settings.FailureCycles != 3 || r.Settings.RecoveryCycles != 8 {
		t.Fatal("defaults not applied")
	}
	if e = ValidateTopology(r.Settings, r.Hardware); e != nil {
		t.Fatal(e)
	}
	r.Settings.ExistingNetworks = append(r.Settings.ExistingNetworks, "192.168.2.128/25")
	if ValidateTopology(r.Settings, r.Hardware) == nil {
		t.Fatal("overlap accepted")
	}
	r.Settings.ExistingNetworks = nil
	r.Hardware.Ports[2].MAC = r.Hardware.Ports[0].MAC
	if ValidateTopology(r.Settings, r.Hardware) == nil {
		t.Fatal("ambiguous MAC accepted")
	}
	// Control-plane load must succeed without ever opening VLESS material.
	_ = os.Remove(filepath.Join(root, "secrets.json"))
	if _, e = LoadControl(root); e != nil {
		t.Fatal(e)
	}
	_ = os.WriteFile(filepath.Join(root, "secrets.json"), []byte(`{"private_canary":"secret-value"}`), 0600)
	_, e = Load(root)
	if e == nil || strings.Contains(e.Error(), "canary") || strings.Contains(e.Error(), "secret-value") {
		t.Fatal("unredacted parse failure", e)
	}
}
