package model

import "time"

type Route string

const (
	RouteGermany     Route = "de"
	RouteNetherlands Route = "nl"
	RouteDirect      Route = "direct"
)

type Mode string

const (
	ModeAuto              Mode = "auto"
	ModePreferGermany     Mode = "prefer-de"
	ModePreferNetherlands Mode = "prefer-nl"
	ModeDirect            Mode = "direct"
)

type HealthState string

const (
	Healthy      HealthState = "healthy"
	Degraded     HealthState = "degraded_fail_open"
	Starting     HealthState = "starting"
	ControlError HealthState = "control_error"
)

type Metric[T any] struct {
	Available bool   `json:"available"`
	Value     *T     `json:"value"`
	Error     string `json:"error,omitempty"`
}

func Available[T any](v T) Metric[T]             { return Metric[T]{Available: true, Value: &v} }
func Unavailable[T any](reason string) Metric[T] { return Metric[T]{Error: reason} }

type ProbeTargetResult struct {
	Success bool          `json:"success"`
	Latency time.Duration `json:"-"`
}
type RouteProbe struct {
	Targets [2]ProbeTargetResult `json:"targets"`
}
type ProbeCycle struct {
	At          time.Time  `json:"at"`
	Germany     RouteProbe `json:"germany"`
	Netherlands RouteProbe `json:"netherlands"`
	Complete    bool       `json:"complete"`
}
type RouteHealth struct {
	Healthy   bool            `json:"healthy"`
	Available bool            `json:"available"`
	LatencyMS Metric[float64] `json:"latency_ms"`
	Failures  int             `json:"consecutive_failures"`
	Successes int             `json:"consecutive_successes"`
}
type RouteChange struct {
	At       time.Time  `json:"at"`
	Previous Route      `json:"previous"`
	Current  Route      `json:"current"`
	Trigger  string     `json:"trigger"`
	Probes   ProbeCycle `json:"probes"`
}
type FailoverSnapshot struct {
	Mode         Mode         `json:"mode"`
	DesiredRoute Route        `json:"desired_route"`
	State        HealthState  `json:"state"`
	ObservedAt   time.Time    `json:"observed_at"`
	Germany      RouteHealth  `json:"germany"`
	Netherlands  RouteHealth  `json:"netherlands"`
	LastChange   *RouteChange `json:"last_route_change"`
}
type Decision struct {
	Accepted bool
	Error    string
	Snapshot FailoverSnapshot
}
type StatusSnapshot struct {
	FailoverSnapshot
	ActiveRoute Metric[Route] `json:"active_route"`
	DNSRoute    Metric[Route] `json:"dns_route"`
	Internet    Metric[bool]  `json:"internet_status"`
	LAN         Metric[bool]  `json:"lan_status"`
	Tunnel      Metric[bool]  `json:"tunnel_status"`
	BypassCount Metric[int]   `json:"bypass_rules"`
	History     []RouteChange `json:"history"`
}
type TrafficSnapshot struct {
	DownloadRate      Metric[uint64] `json:"download_rate"`
	UploadRate        Metric[uint64] `json:"upload_rate"`
	DownloadTotal     Metric[uint64] `json:"download_total"`
	UploadTotal       Metric[uint64] `json:"upload_total"`
	ActiveConnections Metric[int]    `json:"active_connections"`
	TrafficToday      Metric[uint64] `json:"traffic_today"`
	TrafficMonth      Metric[uint64] `json:"traffic_month"`
}
type SystemSnapshot struct {
	Uptime         Metric[float64] `json:"uptime"`
	Temperature    Metric[float64] `json:"temperature"`
	Load           Metric[float64] `json:"load"`
	StorageFree    Metric[uint64]  `json:"storage_free"`
	SingBoxVersion Metric[string]  `json:"sing_box_version"`
}
type ComponentHealth struct {
	Controller Metric[bool] `json:"controller"`
	SingBox    Metric[bool] `json:"sing_box"`
	DNS        Metric[bool] `json:"dns"`
}
