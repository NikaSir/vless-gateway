package config

const (
	Upstream   = "upstream_2_5g"
	Downstream = "downstream_2_5g"
	Management = "management_1g"
)

type Settings struct {
	TransitSubnet       string    `json:"transit_subnet"`
	ManagementAddress   string    `json:"management_address"`
	ManagementSubnet    string    `json:"management_subnet"`
	ManagementListen    string    `json:"management_listen"`
	HAHost              string    `json:"ha_host"`
	AdminSubnet         string    `json:"admin_subnet"`
	ExistingNetworks    []string  `json:"existing_networks"`
	DirectResolver      string    `json:"direct_resolver"`
	EncryptedResolver   string    `json:"encrypted_resolver"`
	EncryptedServerName string    `json:"encrypted_server_name"`
	ClashURL            string    `json:"clash_url"`
	ProbeURLs           [2]string `json:"probe_urls"`
	IntervalSeconds     int       `json:"interval_seconds"`
	TimeoutSeconds      int       `json:"timeout_seconds"`
	BootstrapSuccesses  int       `json:"bootstrap_successes"`
	FailureCycles       int       `json:"failure_cycles"`
	RecoveryCycles      int       `json:"recovery_cycles"`
}
type Port struct {
	Role      string `json:"role"`
	Interface string `json:"interface"`
	MAC       string `json:"mac"`
	Label     string `json:"label"`
}
type Hardware struct {
	Ports []Port `json:"ports"`
}

func (h Hardware) Interface(role string) string {
	for _, p := range h.Ports {
		if p.Role == role {
			return p.Interface
		}
	}
	return ""
}

type Auth struct {
	APIToken   string `json:"api_token"`
	ClashToken string `json:"clash_token"`
}
type Profile struct {
	Server     string `json:"server"`
	ServerPort uint16 `json:"server_port"`
	UUID       string `json:"uuid"`
	ServerName string `json:"server_name"`
	PublicKey  string `json:"public_key"`
	ShortID    string `json:"short_id"`
}
type Secrets struct {
	Germany     Profile `json:"germany"`
	Netherlands Profile `json:"netherlands"`
}
type Runtime struct {
	Root     string
	Settings Settings
	Hardware Hardware
	Auth     Auth
	Secrets  Secrets
}
