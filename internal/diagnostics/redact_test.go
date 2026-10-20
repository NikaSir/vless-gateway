package diagnostics

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactSuppressesUnknownStringsPreservesTypes(t *testing.T) {
	v := Redact(map[string]any{"endpoint": "secret.example/abc", "ip": "192.0.2.44", "token": "canary", "number": 42, "ok": true, "nested": []any{"abcd", false}})
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	for _, secret := range []string{"secret.example", "192.0.2.44", "canary", "abcd"} {
		if strings.Contains(string(b), secret) {
			t.Fatal("secret leaked")
		}
	}
	if !strings.Contains(string(b), `"number":42`) || !strings.Contains(string(b), `"ok":true`) {
		t.Fatal("type lost")
	}
}
