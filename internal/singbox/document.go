package singbox

// Document models the supported sing-box 1.12 configuration surface only.
type Document struct {
	Log          map[string]any `json:"log"`
	DNS          DNS            `json:"dns"`
	Inbounds     []Inbound      `json:"inbounds"`
	Outbounds    []Outbound     `json:"outbounds"`
	Route        Route          `json:"route"`
	Experimental Experimental   `json:"experimental"`
}
type Inbound struct {
	Type               string   `json:"type"`
	Tag                string   `json:"tag"`
	Listen             string   `json:"listen,omitempty"`
	ListenPort         int      `json:"listen_port,omitempty"`
	InterfaceName      string   `json:"interface_name,omitempty"`
	Address            []string `json:"address,omitempty"`
	AutoRoute          bool     `json:"auto_route,omitempty"`
	AutoRedirect       bool     `json:"auto_redirect,omitempty"`
	IncludeInterface   []string `json:"include_interface,omitempty"`
	IPRoute2TableIndex int      `json:"iproute2_table_index,omitempty"`
	IPRoute2RuleIndex  int      `json:"iproute2_rule_index,omitempty"`
	FallbackIndex      int      `json:"auto_redirect_iproute2_fallback_rule_index,omitempty"`
	InputMark          string   `json:"auto_redirect_input_mark,omitempty"`
	OutputMark         string   `json:"auto_redirect_output_mark,omitempty"`
	MTU                int      `json:"mtu,omitempty"`
	Stack              string   `json:"stack,omitempty"`
}
type TLS struct {
	Enabled    bool     `json:"enabled"`
	ServerName string   `json:"server_name"`
	UTLS       *UTLS    `json:"utls,omitempty"`
	Reality    *Reality `json:"reality,omitempty"`
}
type UTLS struct {
	Enabled     bool   `json:"enabled"`
	Fingerprint string `json:"fingerprint"`
}
type Reality struct {
	Enabled   bool   `json:"enabled"`
	PublicKey string `json:"public_key"`
	ShortID   string `json:"short_id"`
}
type Outbound struct {
	Type       string   `json:"type"`
	Tag        string   `json:"tag"`
	Outbounds  []string `json:"outbounds,omitempty"`
	Default    string   `json:"default,omitempty"`
	Server     string   `json:"server,omitempty"`
	ServerPort uint16   `json:"server_port,omitempty"`
	UUID       string   `json:"uuid,omitempty"`
	Flow       string   `json:"flow,omitempty"`
	TLS        *TLS     `json:"tls,omitempty"`
}
type DNSServer struct {
	Type       string `json:"type"`
	Tag        string `json:"tag"`
	Server     string `json:"server"`
	ServerPort int    `json:"server_port,omitempty"`
	Detour     string `json:"detour,omitempty"`
	TLS        *TLS   `json:"tls,omitempty"`
}
type DNSRule struct {
	RuleSet []string `json:"rule_set,omitempty"`
	Action  string   `json:"action"`
	Server  string   `json:"server"`
}
type DNS struct {
	Servers        []DNSServer `json:"servers"`
	Rules          []DNSRule   `json:"rules"`
	Final          string      `json:"final"`
	Strategy       string      `json:"strategy"`
	ReverseMapping bool        `json:"reverse_mapping"`
}
type Rule struct {
	Inbound     []string `json:"inbound,omitempty"`
	IPCIDR      []string `json:"ip_cidr,omitempty"`
	IPIsPrivate bool     `json:"ip_is_private,omitempty"`
	RuleSet     []string `json:"rule_set,omitempty"`
	Action      string   `json:"action"`
	Outbound    string   `json:"outbound,omitempty"`
}
type RuleSet struct {
	Type   string `json:"type"`
	Tag    string `json:"tag"`
	Format string `json:"format"`
	Path   string `json:"path"`
}
type Route struct {
	Rules               []Rule    `json:"rules"`
	RuleSet             []RuleSet `json:"rule_set"`
	Final               string    `json:"final"`
	AutoDetectInterface bool      `json:"auto_detect_interface"`
}
type Experimental struct {
	ClashAPI  ClashAPI  `json:"clash_api"`
	CacheFile CacheFile `json:"cache_file"`
}
type ClashAPI struct {
	ExternalController string `json:"external_controller"`
	Secret             string `json:"secret"`
}
type CacheFile struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
}
