package singbox

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/NikaSir/vless-gateway/internal/clash"
	"github.com/NikaSir/vless-gateway/internal/config"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPinnedSingBoxAcceptsRenderedConfiguration(t *testing.T) {
	binary := os.Getenv("SING_BOX_BIN")
	if binary == "" {
		t.Skip("pinned binary required in CI")
	}
	root := t.TempDir()
	for _, name := range []string{"settings", "hardware", "auth", "secrets"} {
		b, e := os.ReadFile("../../config/examples/" + name + ".example.json")
		if e != nil {
			t.Fatal(e)
		}
		if os.WriteFile(filepath.Join(root, name+".json"), b, 0600) != nil {
			t.Fatal("fixture write")
		}
	}
	r, e := config.Load(root)
	if e != nil {
		t.Fatal(e)
	}
	r.Auth = config.Auth{APIToken: strings.Repeat("a", 40), ClashToken: strings.Repeat("b", 40)}
	for _, p := range []*config.Profile{&r.Secrets.Germany, &r.Secrets.Netherlands} {
		p.UUID = fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", 1, 2, 3, 4, 5)
		p.PublicKey = base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
		p.ShortID = "ab"
		p.ServerName = "example.com"
	}
	dir := filepath.Join(root, "rules", "current")
	_ = os.MkdirAll(dir, 0700)
	src := filepath.Join(root, "rules.json")
	_ = os.WriteFile(src, []byte(`{"version":3,"rules":[]}`), 0600)
	for _, name := range []string{"force-direct", "force-vless", "geosite-ru", "geoip-ru"} {
		cmd := exec.Command(binary, "rule-set", "compile", src, "-o", filepath.Join(dir, name+".srs"))
		if b, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("rule fixture compile: %s", b)
		}
	}
	b, e := Render(r)
	if e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(root, "config.json")
	_ = os.WriteFile(p, b, 0600)
	if e = (ExternalChecker{binary}).Check(context.Background(), p); e != nil {
		cmd := exec.Command(binary, "check", "-c", p)
		out, _ := cmd.CombinedOutput()
		t.Fatalf("pinned checker rejected synthetic config: %s", out)
	}
}

// Run only the selector/cache subsystem: no TUN, VLESS endpoint or external traffic.
// Seed an old persisted choice to prove the generated cold-start settings ignore it.
func TestPinnedSelectorColdStartIgnoresPersistedChoice(t *testing.T) {
	binary := os.Getenv("SING_BOX_BIN")
	if binary == "" {
		t.Skip("pinned binary required in CI")
	}
	for _, choice := range []string{"vless-de", "vless-nl"} {
		t.Run(choice, func(t *testing.T) {
			listener, e := net.Listen("tcp", "127.0.0.1:0")
			if e != nil {
				if os.Getenv("NIKAS_ALLOW_SOCKET_SKIP") == "1" && errors.Is(e, syscall.EPERM) {
					t.Skip("local TCP socket creation denied; CI must run unskipped")
				}
				t.Fatal(e)
			}
			addr := listener.Addr().String()
			listener.Close()
			root := t.TempDir()
			b, e := Render(config.Runtime{Root: root})
			if e != nil {
				t.Fatal(e)
			}
			var generated Document
			if e = json.Unmarshal(b, &generated); e != nil {
				t.Fatal(e)
			}
			cachePath := filepath.Join(root, "sing-box.db")
			doc := map[string]any{"log": map[string]any{"disabled": true}, "outbounds": []Outbound{generated.Outbounds[0], {Type: "direct", Tag: "direct"}, {Type: "direct", Tag: "vless-de"}, {Type: "direct", Tag: "vless-nl"}}, "route": map[string]any{"final": "vless-active"}, "experimental": Experimental{ClashAPI: ClashAPI{addr, "test-secret"}, CacheFile: CacheFile{true, cachePath}}}
			path := filepath.Join(root, "selector.json")
			client, e := clash.New("http://"+addr, "test-secret", &http.Client{Timeout: 100 * time.Millisecond})
			if e != nil {
				t.Fatal(e)
			}
			start := func() *exec.Cmd {
				t.Helper()
				b, e := json.Marshal(doc)
				if e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(path, b, 0600); e != nil {
					t.Fatal(e)
				}
				cmd := exec.Command(binary, "run", "-c", path)
				var log bytes.Buffer
				cmd.Stderr = &log
				if e = cmd.Start(); e != nil {
					t.Fatal(e)
				}
				t.Cleanup(func() {
					if cmd.ProcessState == nil {
						cmd.Process.Kill()
						cmd.Wait()
					}
				})
				for range 100 {
					if version, e := client.Version(context.Background()); e == nil {
						if version != "sing-box 1.12.22" {
							t.Fatalf("unexpected pinned runtime %q", version)
						}
						return cmd
					}
					time.Sleep(20 * time.Millisecond)
				}
				cmd.Process.Kill()
				cmd.Wait()
				if os.Getenv("NIKAS_ALLOW_SOCKET_SKIP") == "1" && strings.Contains(log.String(), "start service: operation not permitted") {
					t.Skip("local sing-box service startup denied; CI must run unskipped")
				}
				t.Fatalf("selector API did not start: %s", log.String())
				return nil
			}
			stop := func(cmd *exec.Cmd) {
				t.Helper()
				if e := cmd.Process.Signal(os.Interrupt); e != nil {
					t.Fatal(e)
				}
				if e := cmd.Wait(); e != nil {
					t.Fatal(e)
				}
			}
			cmd := start()
			if e = client.Select(context.Background(), "vless-active", choice); e != nil {
				t.Fatal(e)
			}
			stop(cmd)
			// Verify the fixture really persisted a choice with the pinned selector implementation.
			cmd = start()
			got, e := client.Selected(context.Background(), "vless-active")
			if e != nil || got != choice {
				t.Fatalf("seeded cache not restored: %s %v", got, e)
			}
			stop(cmd)
			experimental := generated.Experimental
			experimental.ClashAPI = ClashAPI{addr, "test-secret"}
			experimental.CacheFile.Path = cachePath
			doc["experimental"] = experimental
			cmd = start()
			got, e = client.Selected(context.Background(), "vless-active")
			if e != nil || got != "direct" {
				t.Fatalf("cold start selected %s, want direct: %v", got, e)
			}
			stop(cmd)
		})
	}
}
