package clash

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientProtocol(t *testing.T) {
	selected := "direct"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer canary-token" {
			t.Error("auth missing")
		}
		switch r.URL.Path {
		case "/proxies/a/b/delay":
			if r.URL.EscapedPath() != "/proxies/a%2Fb/delay" || r.URL.Query().Get("url") != "https://example.com/204?a=1" || r.URL.Query().Get("timeout") != "2000" {
				t.Error("bad delay request", r.URL)
			}
			_, _ = w.Write([]byte(`{"delay":27}`))
		case "/proxies/vless-active":
			if r.Method == "PUT" {
				var body struct {
					Name string `json:"name"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				selected = body.Name
				w.WriteHeader(204)
			} else {
				_ = json.NewEncoder(w).Encode(map[string]string{"now": selected})
			}
		case "/traffic":
			_, _ = w.Write([]byte("{\"up\":5,\"down\":9}\n"))
		case "/connections":
			_, _ = w.Write([]byte(`{"downloadTotal":51,"uploadTotal":12,"connections":[{"id":"abc"}]}`))
		case "/version":
			_, _ = w.Write([]byte(`{"version":"1.12.22"}`))
		default:
			w.WriteHeader(503)
			_, _ = w.Write([]byte("canary-token raw body"))
		}
	}))
	defer srv.Close()
	c, e := New(srv.URL, "canary-token", srv.Client())
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	d, e := c.Delay(ctx, "a/b", "https://example.com/204?a=1", 2*time.Second)
	if e != nil || d.Latency != 27*time.Millisecond {
		t.Fatal(d, e)
	}
	if e = c.Select(ctx, "vless-active", "vless-de"); e != nil {
		t.Fatal(e)
	}
	got, e := c.Selected(ctx, "vless-active")
	if e != nil || got != "vless-de" {
		t.Fatal(got, e)
	}
	tr, e := c.Traffic(ctx)
	if e != nil || tr.Up != 5 || tr.Down != 9 {
		t.Fatal(tr, e)
	}
	co, e := c.Connections(ctx)
	if e != nil || co.DownloadTotal != 51 || co.Active != 1 {
		t.Fatal(co, e)
	}
	v, e := c.Version(ctx)
	if e != nil || v != "1.12.22" {
		t.Fatal(v, e)
	}
	_, e = c.Selected(ctx, "bad")
	if e == nil || strings.Contains(e.Error(), "canary") {
		t.Fatal("unsafe error", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = c.Version(cancelled); e == nil {
		t.Fatal("cancel ignored")
	}
}
func TestClientRejectsOversizedAndMalformed(t *testing.T) {
	for _, body := range []string{strings.Repeat("x", (1<<20)+1), `{"delay":-1}`, `{`, `{"delay":null}`} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
		c, _ := New(srv.URL, "x", srv.Client())
		_, err := c.Delay(context.Background(), "de", "https://example.com", time.Second)
		srv.Close()
		if err == nil {
			t.Fatal("invalid delay response accepted")
		}
	}
}
