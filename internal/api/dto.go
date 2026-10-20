package api

import (
	"github.com/NikaSir/vless-gateway/internal/model"
	"time"
)

type Freshness struct {
	Available  bool     `json:"available"`
	AgeSeconds *float64 `json:"age_seconds"`
	Stale      bool     `json:"stale"`
}
type Envelope struct {
	SchemaVersion  string            `json:"schema_version"`
	GatewayVersion string            `json:"gateway_version"`
	GeneratedAt    time.Time         `json:"generated_at"`
	State          model.HealthState `json:"state"`
	Freshness      Freshness         `json:"freshness"`
	Data           any               `json:"data"`
	Error          string            `json:"error,omitempty"`
}
