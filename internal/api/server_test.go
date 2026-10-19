package api

import (
	"context"
	"encoding/json"
	"github.com/NikaSir/vless-gateway/internal/buildinfo"
	"github.com/NikaSir/vless-gateway/internal/diagnostics"
	"github.com/NikaSir/vless-gateway/internal/model"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type provider struct{ panicOn bool }

func (p provider) Snapshot(context.Context) model.StatusSnapshot {
	if p.panicOn {
		panic("canary-secret")
	}
	return model.StatusSnapshot{FailoverSnapshot: model.FailoverSnapshot{State: model.Healthy, ObservedAt: time.Now().UTC(), Mode: model.ModeAuto}, ActiveRoute: model.Available(model.RouteGermany)}
}
func (provider) Traffic(context.Context) model.TrafficSnapshot {
	return model.TrafficSnapshot{DownloadRate: model.Unavailable[uint64]("not_measured")}
}
func (provider) System(context.Context) model.SystemSnapshot    { return model.SystemSnapshot{} }
func (provider) Diagnostics(context.Context) diagnostics.Report { return diagnostics.Report{} }
func (provider) Health(context.Context) model.ComponentHealth   { return model.ComponentHealth{} }
func TestAPIReadOnlyAuthAndFreshness(t *testing.T) {
	h, e := NewServer(ServerConfig{ManagementListen: "192.168.10.5:9080", BearerToken: strings.Repeat("t", 32), LoopbackLiveness: true}, provider{}, buildinfo.Current())
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		method, path, token, remote string
		code                        int
	}{{"GET", "/api/v1/status", "", "192.168.10.10:22", 401}, {"GET", "/api/v1/status", "wrong", "192.168.10.10:22", 401}, {"POST", "/api/v1/status", strings.Repeat("t", 32), "192.168.10.10:22", 405}, {"GET", "/api/v1/unknown", strings.Repeat("t", 32), "192.168.10.10:22", 404}, {"GET", "/livez", "", "127.0.0.1:55", 200}, {"GET", "/livez", "", "192.168.10.10:22", 401}} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.RemoteAddr = tc.remote
		if tc.token != "" {
			req.Header.Set("Authorization", "Bearer "+tc.token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != tc.code {
			t.Fatalf("%s %s %d", tc.method, tc.path, w.Code)
		}
	}
	for _, ep := range []string{"status", "routes", "traffic", "system", "diagnostics", "health"} {
		req := httptest.NewRequest("GET", "/api/v1/"+ep, nil)
		req.Header.Set("Authorization", "Bearer "+strings.Repeat("t", 32))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		var v map[string]any
		if json.Unmarshal(w.Body.Bytes(), &v) != nil || v["schema_version"] != "1" || v["state"] != "healthy" || v["freshness"] == nil || v["generated_at"] == nil || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(ep, w.Body.String())
		}
	}
}
func TestAPIPanicDoesNotLeak(t *testing.T) {
	h, _ := NewServer(ServerConfig{ManagementListen: "192.168.10.5:9080", BearerToken: strings.Repeat("t", 32)}, provider{true}, buildinfo.Current())
	r := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("t", 32))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 500 || strings.Contains(w.Body.String(), "canary") {
		t.Fatal(w.Body.String())
	}
}
