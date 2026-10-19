package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/NikaSir/vless-gateway/internal/buildinfo"
	"github.com/NikaSir/vless-gateway/internal/diagnostics"
	"github.com/NikaSir/vless-gateway/internal/model"
	"net"
	"net/http"
	"strings"
	"time"
)

type Provider interface {
	Snapshot(context.Context) model.StatusSnapshot
	Traffic(context.Context) model.TrafficSnapshot
	System(context.Context) model.SystemSnapshot
	Diagnostics(context.Context) diagnostics.Report
	Health(context.Context) model.ComponentHealth
}
type ServerConfig struct {
	ManagementListen, BearerToken string
	LoopbackLiveness              bool
}

func NewServer(c ServerConfig, p Provider, info buildinfo.Info) (http.Handler, error) {
	host, _, e := net.SplitHostPort(c.ManagementListen)
	ip := net.ParseIP(host)
	if e != nil || ip == nil || ip.IsUnspecified() || len(c.BearerToken) < 32 || p == nil {
		return nil, errors.New("invalid API configuration")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		var id [12]byte
		if _, err := rand.Read(id[:]); err == nil {
			w.Header().Set("X-Request-ID", hex.EncodeToString(id[:]))
		}
		defer func() {
			if recover() != nil {
				w.WriteHeader(500)
				_, _ = w.Write([]byte(`{"error":"internal_error"}`))
			}
		}()
		if len(r.Header.Get("Authorization")) > 4096 {
			w.WriteHeader(431)
			return
		}
		if r.URL.Path == "/livez" && c.LoopbackLiveness {
			remote, _, _ := net.SplitHostPort(r.RemoteAddr)
			if addr := net.ParseIP(remote); addr != nil && addr.IsLoopback() {
				if r.Method != "GET" {
					w.WriteHeader(405)
					return
				}
				_, _ = w.Write([]byte(`{"alive":true}`))
				return
			}
		}
		if !authorized(r.Header.Get("Authorization"), c.BearerToken) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		if r.Method != "GET" {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(405)
			_, _ = w.Write([]byte(`{"error":"read_only"}`))
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			w.WriteHeader(404)
			return
		}
		ep := strings.TrimPrefix(r.URL.Path, "/api/v1/")
		switch ep {
		case "status", "routes", "traffic", "system", "diagnostics", "health":
		default:
			w.WriteHeader(404)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		snap := p.Snapshot(ctx)
		now := time.Now().UTC()
		f := Freshness{Stale: true}
		if !snap.ObservedAt.IsZero() {
			age := now.Sub(snap.ObservedAt).Seconds()
			if age < 0 {
				age = 0
			}
			f = Freshness{Available: true, AgeSeconds: &age, Stale: age > 45}
		}
		var data any
		switch ep {
		case "status":
			data = snap
		case "routes":
			data = snap.FailoverSnapshot
		case "traffic":
			data = p.Traffic(ctx)
		case "system":
			data = p.System(ctx)
		case "diagnostics":
			data = p.Diagnostics(ctx)
		case "health":
			data = p.Health(ctx)
		}
		state := snap.State
		if state == "" {
			state = model.Starting
		}
		b, err := json.Marshal(Envelope{SchemaVersion: info.APISchemaVersion, GatewayVersion: info.GatewayVersion, GeneratedAt: now, State: state, Freshness: f, Data: data})
		if err != nil || len(b) > 1<<20 {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"error":"response_unavailable"}`))
			return
		}
		_, _ = w.Write(b)
	}), nil
}
