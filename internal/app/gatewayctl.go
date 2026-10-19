package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/NikaSir/vless-gateway/internal/backup"
	"github.com/NikaSir/vless-gateway/internal/buildinfo"
	"github.com/NikaSir/vless-gateway/internal/config"
	"github.com/NikaSir/vless-gateway/internal/control"
	"github.com/NikaSir/vless-gateway/internal/provision"
	"github.com/NikaSir/vless-gateway/internal/rules"
	"github.com/NikaSir/vless-gateway/internal/singbox"
	"github.com/NikaSir/vless-gateway/internal/state"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

const DefaultRoot = "/etc/nikas-vless"

func SocketPath(root string) string {
	if root == DefaultRoot {
		return "/var/run/nikas-vless/gatewayd.sock"
	}
	return filepath.Join(root, "state", "gatewayd.sock")
}
func RunCLI(ctx context.Context, args []string, out io.Writer, uid int) error {
	fs := flag.NewFlagSet("gatewayctl", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := DefaultRoot
	fs.StringVar(&root, "config-root", root, "")
	fs.StringVar(&root, "root", root, "")
	version := fs.Bool("version", false, "")
	offline := fs.Bool("offline", false, "")
	if fs.Parse(args) != nil {
		return errors.New("invalid command options")
	}
	if *version {
		return json.NewEncoder(out).Encode(buildinfo.Current())
	}
	a := fs.Args()
	if len(a) == 0 {
		return errors.New("explicit subcommand required")
	}
	command := a[0]
	switch command {
	case "status", "diagnostics":
		if len(a) != 1 {
			return errors.New("unexpected command arguments")
		}
	case "set-mode":
		if len(a) != 2 {
			return errors.New("mode required")
		}
		if uid != 0 {
			return errors.New("root required")
		}
		switch a[1] {
		case "auto", "prefer-de", "prefer-nl", "direct":
		default:
			return errors.New("invalid mode")
		}
	case "validate", "render", "reload", "rules-update", "backup", "restore", "doctor", "preflight", "init-auth", "validate-input":
		if uid != 0 {
			return errors.New("root required")
		}
	default:
		return errors.New("unknown command")
	}
	if command == "preflight" {
		pf := flag.NewFlagSet("preflight", flag.ContinueOnError)
		pf.SetOutput(io.Discard)
		inventory := pf.String("inventory", "", "")
		output := pf.String("output", "", "")
		owned := pf.Bool("owned", false, "")
		if pf.Parse(a[1:]) != nil {
			return errors.New("invalid preflight options")
		}
		var s config.Settings
		var h config.Hardware
		if config.ReadJSON(filepath.Join(root, "settings.json"), &s) != nil || config.ReadJSON(filepath.Join(root, "hardware.json"), &h) != nil {
			return errors.New("preflight settings unavailable")
		}
		var i provision.Inventory
		var err error
		if *inventory != "" {
			err = config.ReadJSON(*inventory, &i)
		} else {
			i, err = provision.Inspect(ctx)
		}
		if err != nil {
			return err
		}
		files, err := provision.Build(s, h, i, *owned)
		if err != nil {
			return err
		}
		if *output != "" {
			for path, b := range files {
				if err = state.AtomicWrite(filepath.Join(*output, path), b, 0600); err != nil {
					return err
				}
			}
		}
		_, err = fmt.Fprintln(out, "preflight validated")
		return err
	}
	if command == "validate-input" {
		r, e := config.Load(root)
		if e != nil {
			return e
		}
		return config.Validate(r)
	}
	if command == "init-auth" {
		path := filepath.Join(root, "auth.json")
		if _, err := os.Lstat(path); err == nil {
			return errors.New("auth already exists")
		}
		var a, b [32]byte
		if _, err := rand.Read(a[:]); err != nil {
			return errors.New("random generator failed")
		}
		if _, err := rand.Read(b[:]); err != nil {
			return errors.New("random generator failed")
		}
		data, _ := json.Marshal(config.Auth{APIToken: hex.EncodeToString(a[:]), ClashToken: hex.EncodeToString(b[:])})
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return errors.New("auth creation failed")
		}
		defer f.Close()
		if _, err = f.Write(data); err != nil {
			return errors.New("auth write failed")
		}
		return f.Sync()
	}
	if command == "backup" || command == "restore" {
		if command == "restore" {
			yes := false
			backup := false
			for i, v := range a[1:] {
				if v == "--yes" {
					yes = true
				}
				if v == "--backup" && i+2 < len(a) {
					backup = true
				}
			}
			if !yes || !backup {
				return errors.New("restore requires --backup and --yes")
			}
		}
		bf := flag.NewFlagSet(command, flag.ContinueOnError)
		bf.SetOutput(io.Discard)
		systemRoot := bf.String("system-root", "/", "")
		archive := bf.String("backup", "", "")
		yes := bf.Bool("yes", false, "")
		if bf.Parse(a[1:]) != nil {
			return errors.New("invalid backup options")
		}
		if command == "backup" {
			path, e := backup.Create(*systemRoot, filepath.Join(*systemRoot, "etc/nikas-vless/backups"))
			if e != nil {
				return e
			}
			_, e = fmt.Fprintln(out, path)
			return e
		}
		if !*yes || *archive == "" {
			return errors.New("restore requires --backup and --yes")
		}
		if *systemRoot == "/" {
			i, e := provision.Inspect(ctx)
			if e != nil || i.Board != "friendlyarm,nanopi-r5s" || i.Version != "24.10.2" {
				return errors.New("restore device verification failed")
			}
			_ = exec.CommandContext(ctx, "/etc/init.d/nikas-sing-box", "stop").Run()
			_ = exec.CommandContext(ctx, "/etc/init.d/nikas-gatewayd", "stop").Run()
		}
		if e := backup.Restore(*systemRoot, *archive); e != nil {
			return e
		}
		if *systemRoot == "/" {
			_ = exec.CommandContext(ctx, "uci", "set", "nikas-vless.main.policy_enabled=0").Run()
			_ = exec.CommandContext(ctx, "uci", "commit", "nikas-vless").Run()
			_ = exec.CommandContext(ctx, "/etc/init.d/network", "restart").Run()
			_ = exec.CommandContext(ctx, "/etc/init.d/firewall", "restart").Run()
			_ = exec.CommandContext(ctx, "/etc/init.d/dnsmasq", "restart").Run()
			_ = exec.CommandContext(ctx, "/etc/init.d/nikas-gatewayd", "restart").Run()
		}
		_, e := fmt.Fprintln(out, "restored; DIRECT stage required")
		return e
	}
	if command == "doctor" {
		resp, e := control.Call(ctx, SocketPath(root), control.Request{Command: "diagnostics"})
		if e != nil {
			return e
		}
		return json.NewEncoder(out).Encode(resp.Data)
	}
	if command == "render" || command == "validate" || *offline {
		switch command {
		case "render", "validate", "reload", "rules-update":
		default:
			return errors.New("unsupported offline command")
		}
		r, e := config.Load(root)
		if e != nil {
			return e
		}
		if e = config.Validate(r); e != nil {
			return e
		}
		checker := singbox.ExternalChecker{Binary: "sing-box"}
		reloader := NewReloader(r)
		if command == "rules-update" {
			u := rules.Updater{Root: filepath.Join(root, "rules"), Overrides: filepath.Join(root, "overrides"), Fetcher: rules.HTTPFetcher{}, Tool: RuleTool{Root: root, Checker: checker}, Activator: reloader}
			m, e := u.Update(ctx)
			if e != nil {
				return e
			}
			return json.NewEncoder(out).Encode(m)
		}
		data, e := singbox.Render(r)
		if e != nil {
			return errors.New("render failed")
		}
		if command == "render" {
			if len(a) != 3 || a[1] != "--output" {
				return errors.New("render requires --output file")
			}
			return state.AtomicWrite(a[2], data, 0600)
		}
		if command == "reload" {
			return singbox.Activate(ctx, filepath.Join(root, "config.json"), data, checker, reloader)
		}
		temp, e := os.CreateTemp(root, ".validate-")
		if e != nil {
			return errors.New("validation staging failed")
		}
		p := temp.Name()
		_ = temp.Close()
		defer os.Remove(p)
		if state.AtomicWrite(p, data, 0600) != nil {
			return errors.New("validation staging failed")
		}
		if e = checker.Check(ctx, p); e != nil {
			return e
		}
		_, e = fmt.Fprintln(out, "validated")
		return e
	}
	req := control.Request{Command: command}
	if command == "set-mode" {
		req.Mode = a[1]
	}
	resp, e := control.Call(ctx, SocketPath(root), req)
	if e != nil {
		return e
	}
	return json.NewEncoder(out).Encode(resp.Data)
}

var mutationMu sync.Mutex
