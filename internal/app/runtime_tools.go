package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/NikaSir/vless-gateway/internal/clash"
	"github.com/NikaSir/vless-gateway/internal/config"
	"github.com/NikaSir/vless-gateway/internal/singbox"
	"github.com/NikaSir/vless-gateway/internal/state"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Reloader struct {
	root   string
	client clash.Client
}

func NewReloader(r config.Runtime) *Reloader {
	c, _ := clash.New(r.Settings.ClashURL, r.Auth.ClashToken, &http.Client{Timeout: 2 * time.Second})
	return &Reloader{r.Root, c}
}
func (r *Reloader) Reload(ctx context.Context) error {
	if exec.CommandContext(ctx, "/etc/init.d/nikas-sing-box", "restart").Run() != nil {
		return errors.New("sing-box restart failed")
	}
	return nil
}
func (r *Reloader) Healthy(ctx context.Context) error {
	for i := 0; i < 20; i++ {
		if r.client != nil {
			v, e := r.client.Version(ctx)
			if e == nil && v == "1.12.22" {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	return errors.New("sing-box health not confirmed")
}
func (r *Reloader) ReloadAndCheck(ctx context.Context) error {
	if e := r.Reload(ctx); e != nil {
		return e
	}
	return r.Healthy(ctx)
}

type RuleTool struct {
	Root    string
	Checker singbox.ExternalChecker
}

func (t RuleTool) ValidateBinary(ctx context.Context, path string) error {
	if exec.CommandContext(ctx, "sing-box", "rule-set", "decompile", path, "-o", os.DevNull).Run() != nil {
		return errors.New("invalid binary rules")
	}
	return nil
}
func (t RuleTool) CompileSource(ctx context.Context, src, dst string) error {
	if exec.CommandContext(ctx, "sing-box", "rule-set", "compile", src, "-o", dst).Run() != nil {
		return errors.New("invalid local overrides")
	}
	return nil
}
func (t RuleTool) CheckConfig(ctx context.Context, generation string) error {
	b, e := os.ReadFile(filepath.Join(t.Root, "config.json"))
	if e != nil {
		return errors.New("active config unavailable")
	}
	var d singbox.Document
	if json.Unmarshal(b, &d) != nil {
		return errors.New("active config invalid")
	}
	for i := range d.Route.RuleSet {
		d.Route.RuleSet[i].Path = filepath.Join(generation, d.Route.RuleSet[i].Tag+".srs")
	}
	b, e = json.Marshal(d)
	if e != nil {
		return errors.New("staged config encoding failed")
	}
	p := filepath.Join(generation, ".check-config.json")
	defer os.Remove(p)
	if state.AtomicWrite(p, b, 0600) != nil {
		return errors.New("staged config unavailable")
	}
	return t.Checker.Check(ctx, p)
}
