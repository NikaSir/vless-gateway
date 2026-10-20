package diagnostics

import (
	"github.com/NikaSir/vless-gateway/internal/model"
)

// Report is an allow-list: it cannot contain runtime config, destinations or tokens.
type Report struct {
	State      model.HealthState         `json:"state"`
	Mode       model.Mode                `json:"mode"`
	Route      model.Metric[model.Route] `json:"active_route"`
	Components model.ComponentHealth     `json:"components"`
	Issues     []string                  `json:"issues"`
}

// Redact discards every free-text value; only typed diagnostic fields are trusted.
func Redact(v any) any {
	switch x := v.(type) {
	case string:
		return "[redacted]"
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, v := range x {
			m[k] = Redact(v)
		}
		return m
	case []any:
		a := make([]any, len(x))
		for i, v := range x {
			a[i] = Redact(v)
		}
		return a
	default:
		return v
	}
}
