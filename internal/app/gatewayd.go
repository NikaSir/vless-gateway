package app

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"github.com/NikaSir/vless-gateway/internal/api"
	"github.com/NikaSir/vless-gateway/internal/buildinfo"
	"github.com/NikaSir/vless-gateway/internal/clash"
	"github.com/NikaSir/vless-gateway/internal/config"
	"github.com/NikaSir/vless-gateway/internal/control"
	"github.com/NikaSir/vless-gateway/internal/controller"
	"github.com/NikaSir/vless-gateway/internal/diagnostics"
	"github.com/NikaSir/vless-gateway/internal/failover"
	"github.com/NikaSir/vless-gateway/internal/model"
	"github.com/NikaSir/vless-gateway/internal/state"
	"github.com/NikaSir/vless-gateway/internal/system"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type GatewayProvider struct {
	Controller  *controller.Service
	Clash       clash.Client
	Collector   system.Collector
	LocalDNS    string
	FallbackDNS string
}

func newGatewayProvider(svc *controller.Service, client clash.Client, r config.Runtime) *GatewayProvider {
	return &GatewayProvider{Controller: svc, Clash: client, Collector: system.New(r.Root), LocalDNS: "127.0.0.1:5353", FallbackDNS: net.JoinHostPort(r.Settings.DirectResolver, "53")}
}

func (p *GatewayProvider) Snapshot(ctx context.Context) model.StatusSnapshot {
	s := p.Controller.Snapshot()
	if p.LocalDNS != "" {
		if !dnsHealthy(ctx, p.LocalDNS) {
			s.State = model.Degraded
			s.DNSRoute = model.Unavailable[model.Route]("dns_unavailable")
			if p.FallbackDNS != "" && dnsHealthy(ctx, p.FallbackDNS) {
				s.DNSRoute = model.Available(model.RouteDirect)
			}
		}
	}
	return s
}
func (p *GatewayProvider) Traffic(ctx context.Context) model.TrafficSnapshot {
	s := model.TrafficSnapshot{DownloadRate: model.Unavailable[uint64]("not_available"), UploadRate: model.Unavailable[uint64]("not_available"), DownloadTotal: model.Unavailable[uint64]("not_available"), UploadTotal: model.Unavailable[uint64]("not_available"), ActiveConnections: model.Unavailable[int]("not_available"), TrafficToday: model.Unavailable[uint64]("accounting_not_available"), TrafficMonth: model.Unavailable[uint64]("accounting_not_available")}
	if p.Clash == nil {
		return s
	}
	tr, e := p.Clash.Traffic(ctx)
	if e == nil {
		s.DownloadRate = model.Available(tr.Down)
		s.UploadRate = model.Available(tr.Up)
	}
	co, e := p.Clash.Connections(ctx)
	if e == nil {
		s.DownloadTotal = model.Available(co.DownloadTotal)
		s.UploadTotal = model.Available(co.UploadTotal)
		s.ActiveConnections = model.Available(co.Active)
	}
	return s
}
func (p *GatewayProvider) System(ctx context.Context) model.SystemSnapshot {
	s := p.Collector.Collect()
	if p.Clash != nil {
		v, e := p.Clash.Version(ctx)
		if e == nil {
			s.SingBoxVersion = model.Available(v)
		}
	}
	return s
}
func (p *GatewayProvider) Health(ctx context.Context) model.ComponentHealth {
	s := model.ComponentHealth{Controller: model.Available(true), SingBox: model.Unavailable[bool]("not_available"), DNS: model.Unavailable[bool]("not_measured")}
	if p.Clash != nil {
		_, e := p.Clash.Version(ctx)
		s.SingBox = model.Available(e == nil)
	}
	if p.LocalDNS != "" {
		s.DNS = model.Available(dnsHealthy(ctx, p.LocalDNS))
	}
	return s
}
func (p *GatewayProvider) Diagnostics(ctx context.Context) diagnostics.Report {
	s := p.Snapshot(ctx)
	return diagnostics.Report{State: s.State, Mode: s.Mode, Route: s.ActiveRoute, Components: p.Health(ctx), Issues: []string{}}
}
func dnsHealthy(ctx context.Context, address string) bool {
	d := net.Dialer{Timeout: time.Second}
	c, e := d.DialContext(ctx, "udp", address)
	if e != nil {
		return false
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(time.Second))
	query := []byte{0, 0, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0, 7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 3, 'c', 'o', 'm', 0, 0, 1, 0, 1}
	if _, e = rand.Read(query[:2]); e != nil {
		return false
	}
	if _, e = c.Write(query); e != nil {
		return false
	}
	b := make([]byte, 4096)
	n, e := c.Read(b)
	return e == nil && n >= 12 && b[0] == query[0] && b[1] == query[1] && b[2]&0x80 != 0 && b[3]&15 == 0 && binary.BigEndian.Uint16(b[6:8]) > 0
}
func RunDaemon(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("gatewayd", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := DefaultRoot
	fs.StringVar(&root, "config-root", root, "")
	fs.StringVar(&root, "root", root, "")
	version := fs.Bool("version", false, "")
	if fs.Parse(args) != nil || len(fs.Args()) != 0 {
		return errors.New("invalid gatewayd options")
	}
	if *version {
		return json.NewEncoder(out).Encode(buildinfo.Current())
	}
	// Only public settings/hardware plus API/Clash authentication are loaded here.
	r, e := config.LoadControl(root)
	if e != nil {
		return e
	}
	if e = config.ValidateControl(r); e != nil {
		return e
	}
	client, e := clash.New(r.Settings.ClashURL, r.Auth.ClashToken, nil)
	if e != nil {
		return e
	}
	machine, e := failover.New(failover.Config{BootstrapSuccesses: r.Settings.BootstrapSuccesses, FailureCycles: r.Settings.FailureCycles, RecoveryCycles: r.Settings.RecoveryCycles})
	if e != nil {
		return e
	}
	store := state.New(filepath.Join(root, "state"))
	svc, e := controller.New(controller.Config{Interval: time.Duration(r.Settings.IntervalSeconds) * time.Second, Timeout: time.Duration(r.Settings.TimeoutSeconds) * time.Second, Targets: r.Settings.ProbeURLs}, client, client, machine, store, controller.RealClock{})
	if e != nil {
		return e
	}
	if saved, e := store.Load(ctx); e == nil {
		svc.Restore(saved)
	}
	provider := newGatewayProvider(svc, client, r)
	handler, e := api.NewServer(api.ServerConfig{ManagementListen: r.Settings.ManagementListen, BearerToken: r.Auth.APIToken}, provider, buildinfo.Current())
	if e != nil {
		return e
	}
	httpListener, e := net.Listen("tcp", r.Settings.ManagementListen)
	if e != nil {
		return errors.New("management listener unavailable")
	}
	defer httpListener.Close()
	socket, e := control.Listen(SocketPath(root))
	if e != nil {
		return e
	}
	defer socket.Close()
	defer os.Remove(SocketPath(root))
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	errorsCh := make(chan error, 3)
	go func() { errorsCh <- server.Serve(httpListener) }()
	go func() { errorsCh <- svc.Run(runCtx) }()
	go func() {
		errorsCh <- control.Serve(runCtx, socket, func(c context.Context, req control.Request) control.Response {
			switch req.Command {
			case "status":
				return control.Response{OK: true, Data: provider.Snapshot(c)}
			case "diagnostics":
				return control.Response{OK: true, Data: provider.Diagnostics(c)}
			case "set-mode":
				mutationMu.Lock()
				defer mutationMu.Unlock()
				if svc.SetMode(c, model.Mode(req.Mode)) != nil {
					return control.Response{Error: "mode_change_failed"}
				}
				return control.Response{OK: true, Data: svc.Snapshot()}
			default:
				mutationMu.Lock()
				defer mutationMu.Unlock()
				cmd := exec.CommandContext(c, "/usr/bin/gatewayctl", "--config-root", root, "--offline", req.Command)
				if cmd.Run() != nil {
					return control.Response{Error: "maintenance_failed"}
				}
				return control.Response{OK: true}
			}
		})
	}()
	select {
	case <-ctx.Done():
	case e = <-errorsCh:
	}
	cancel()
	shutdown, c := context.WithTimeout(context.Background(), 3*time.Second)
	defer c()
	_ = server.Shutdown(shutdown)
	if e != nil && !errors.Is(e, http.ErrServerClosed) {
		return errors.New("gateway service stopped")
	}
	return nil
}
