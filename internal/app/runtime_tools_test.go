package app

import (
	"context"
	"github.com/NikaSir/vless-gateway/internal/clash"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReloaderAcceptsPinnedUpstreamVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"version":"sing-box 1.12.22","meta":true}`))
	}))
	defer server.Close()
	client, err := clash.New(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := (&Reloader{client: client}).Healthy(ctx); err != nil {
		t.Fatal(err)
	}
}
