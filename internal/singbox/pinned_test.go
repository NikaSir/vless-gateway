package singbox

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/NikaSir/vless-gateway/internal/config"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
