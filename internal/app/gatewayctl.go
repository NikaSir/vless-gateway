package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/NikaSir/vless-gateway/internal/buildinfo"
	"github.com/NikaSir/vless-gateway/internal/config"
	"github.com/NikaSir/vless-gateway/internal/control"
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
	case "validate", "render", "reload", "rules-update", "backup", "restore", "doctor":
		if uid != 0 {
			return errors.New("root required")
		}
	default:
		return errors.New("unknown command")
	}
	if command == "backup" || command == "restore" || command == "doctor" {
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
		cmd := exec.CommandContext(ctx, "/usr/libexec/nikas-vless/"+command+".sh", a[1:]...)
		cmd.Stdout = out
		if cmd.Run() != nil {
			return errors.New("local maintenance failed")
		}
		return nil
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
