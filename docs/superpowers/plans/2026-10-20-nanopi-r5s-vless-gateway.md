# NanoPi R5S VLESS Gateway Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Реализовать воспроизводимый, отказоустойчивый и откатываемый шлюз версии 1.0.0-beta001 для NanoPi R5S LTS на FriendlyWrt 24.10.2: российские ресурсы идут DIRECT, остальные — строго Germany → Netherlands → DIRECT, а Home Assistant получает только фактическую read-only телеметрию.

**Architecture:** sing-box 1.12.22 остаётся data plane и управляет TUN/auto-route/auto-redirect; статический ARM64-сервис gatewayd является control plane, проверяет оба VLESS-маршрута, управляет Selector через loopback Clash API и публикует аутентифицированный API v1. FriendlyWrt/procd/fw4/dnsmasq обеспечивают загрузку, изоляцию, DNS fallback и fail-open очистку перехвата. gatewayctl использует root-only Unix socket для локальных команд; публичных write endpoint нет.

**Tech Stack:** Go 1.27.2 (stdlib-first), Linux ARM64, sing-box 1.12.22, FriendlyWrt 24.10.2/OpenWrt procd + UCI + firewall4/nftables + dnsmasq, POSIX/BusyBox ash, JSON/JSON Schema, GitHub Actions; для Home Assistant — Python, aiohttp и DataUpdateCoordinator из поддерживаемой версии Home Assistant.

**Spec:** [Approved NanoPi R5S VLESS Gateway design](../specs/2026-10-20-nanopi-r5s-vless-gateway-design.md)

## Global Constraints

- Поддерживается только NanoPi R5S LTS 4 GB / 64 GB eMMC и FriendlyWrt 24.10.2. Не добавлять Raspberry Pi, Debian/Armbian, systemd, контейнерный или IPv6-транзитный путь.
- **No SNAT/masquerade:** NanoPi не создаёт SNAT, masquerade или второй NAT-контур. UXG-Max сохраняет LAN/VLAN, DHCP, Wi-Fi policy и существующий NAT.
- Начальная версия — 1.0.0-beta001; sing-box закреплён на 1.12.22. Обновление этих границ требует отдельного решения и повторного bench-теста.
- Продуктовые UUID, REALITY keys, short IDs, адреса VLESS-серверов, subscription URL и API tokens никогда не попадают в Git, тестовые логи, ошибки, артефакты или диагностику.
- Все изменяющие операции должны быть атомарными, сохранять последнюю рабочую генерацию и иметь проверяемый rollback.
- Любая ошибка VLESS, gatewayd, sing-box, split DNS или rule-set должна сохранять доступ в интернет через DIRECT, если NanoPi и оба кабеля физически работают.
- gatewayd не сбрасывает Selector при штатном завершении: падение контроллера не должно останавливать data plane.
- Публичный API v1 — только GET. Локальные write-команды доступны только root через Unix socket режима 0600.
- Каждый production-code шаг начинается с падающего теста. После каждого task выполняются указанные проверки и создаётся отдельный commit.
- Реальная установка на NanoPi, переключение кабелей и изменение Keenetic/UXG-Max выполняются только после отдельного явного подтверждения владельца и прохождения repository/bench gates.

## Review Focus

1. **Неверная топология до мутаций:** tests Task 10 обязаны отклонять пересекающийся transit subnet, повторяющийся/неизвестный MAC и неоднозначную роль порта до backup или UCI write.
2. **Флаппинг и несогласованные пробы:** tests Tasks 2 и 4 обязаны покрывать bootstrap 2, failure 3, recovery 8, обе probe URL, устаревшие/out-of-order циклы и строгий порядок DE → NL → DIRECT.
3. **Падение sing-box без black hole:** tests Task 9 обязаны доказать, что wrapper удаляет только зарезервированные nft/ip-rule/ip-route артефакты, dnsmasq остаётся доступен и переходит к Keenetic resolver.
4. **Граница API и секреты:** tests Task 7 обязаны покрывать 401/404/405, constant-time bearer check, loopback-only liveness, unavailable вместо выдуманных нулей и отсутствие секретов в response/log/error.
5. **Прерванное обновление:** tests Tasks 5, 6 и 10 обязаны моделировать failed check, failed reload, process interruption и повреждённый manifest; активные config/rules и rollback backup должны остаться рабочими.

---

## Target file map

Каждый файл имеет одну основную ответственность:

- **go.mod, go.sum, VERSION, Makefile** — модуль, версии и единые developer/CI команды.
- **cmd/gatewayd/main.go** — только composition root демона.
- **cmd/gatewayctl/main.go** — только parsing CLI и exit codes.
- **internal/buildinfo/** — версия binary/API schema.
- **internal/model/** — общие immutable domain DTO.
- **internal/failover/** — чистая state machine без I/O.
- **internal/clash/** — loopback Clash API client.
- **internal/controller/** — probe loop, Selector reconciliation и snapshot.
- **internal/state/** — атомарное хранение несекретного состояния и route history.
- **internal/config/** — загрузка и semantic validation трёх runtime JSON.
- **internal/singbox/** — typed document, render, external check и atomic activation.
- **internal/rules/** — staging, validation, generation switch и rollback rule-set.
- **internal/api/** — authenticated read-only HTTP API v1.
- **internal/diagnostics/** — allow-list diagnostics и redaction.
- **internal/system/** — фактические Linux metrics с available/error.
- **internal/control/** — root-only Unix control protocol.
- **internal/app/** — wiring, lifecycle и command handlers, без flag parsing.
- **config/examples/** — только безопасные placeholder examples.
- **config/schema/** — JSON Schema contracts.
- **openwrt/** — rootfs overlay и шаблоны FriendlyWrt.
- **scripts/** — install/backup/restore/doctor/package/verification entrypoints.
- **tests/** — cross-package, overlay, packaging, contract и runbook checks.
- **docs/** — API, deployment, rollback, acceptance и operations.
- **NikaSir/ha-vless-gateway** — отдельный consumer repository; gateway credentials/config туда не копируются.

## Task 1: Establish the Go, version and quality baseline

**Files:**
- Create: **go.mod**
- Create: **VERSION**
- Create: **Makefile**
- Create: **internal/buildinfo/buildinfo_test.go**
- Create: **internal/buildinfo/buildinfo.go**
- Modify: **.gitignore**
- Modify: **.github/dependabot.yml**

- [ ] Add go.mod with module github.com/NikaSir/vless-gateway and Go 1.27.2; create VERSION containing exactly 1.0.0-beta001.
- [ ] Write buildinfo tests first. Required API:

        type Info struct {
            GatewayVersion   string
            APISchemaVersion string
            GoVersion        string
        }
        var Version = "dev"
        const APISchemaVersion = "1"
        func Current() Info
        func ValidateReleaseVersion(string) error

  Tests require the approved beta format, reject whitespace/leading v, and verify that a release build can inject VERSION through -ldflags.
- [ ] Run:

        go test ./internal/buildinfo

  Expected: FAIL because buildinfo.go and symbols do not exist.
- [ ] Implement buildinfo.go, Makefile targets fmt, vet, test, test-race, build-arm64, verify and clean. VERSION is the release source of truth; binaries receive it with -X github.com/NikaSir/vless-gateway/internal/buildinfo.Version.
- [ ] Run:

        make fmt
        make vet
        make test

  Expected: PASS. build-arm64 is defined but is not invoked until Task 8 creates both command packages.
- [ ] Commit:

        git add go.mod VERSION Makefile internal/buildinfo .gitignore .github/dependabot.yml
        git commit -m "build: establish gateway Go baseline"

## Task 2: Implement the pure failover state machine

**Files:**
- Create: **internal/model/types.go**
- Create: **internal/failover/machine_test.go**
- Create: **internal/failover/machine.go**

- [ ] Define shared domain types first: RouteGermany, RouteNetherlands, RouteDirect; ModeAuto, ModePreferGermany, ModePreferNetherlands, ModeDirect; ProbeTargetResult; RouteProbe; ProbeCycle; RouteChange; HealthState; StatusSnapshot. A route cycle is successful when at least one of its two targets succeeds; it fails only when both targets fail.
- [ ] Write table-driven tests for this public surface:

        type Config struct {
            BootstrapSuccesses int
            FailureCycles      int
            RecoveryCycles     int
        }
        func New(Config) (*Machine, error)
        func (m *Machine) Observe(model.ProbeCycle) model.Decision
        func (m *Machine) SetMode(model.Mode) error
        func (m *Machine) Snapshot() model.FailoverSnapshot

  Cover startup DIRECT for one success, Germany on the second; Germany failure on the third consecutive failed cycle; Netherlands fallback; both failed to DIRECT/degraded_fail_open; recovery only on the eighth consecutive successful cycle; auto/prefer modes; direct mode; counter reset; exact RouteChange fields.
- [ ] Add tests that a duplicate timestamp and an older cycle are ignored, incomplete cycles are rejected, and latency equals the minimum successful target latency rather than a fabricated average.
- [ ] Run:

        go test ./internal/failover -run TestMachine -count=1

  Expected: FAIL because Machine is not implemented.
- [ ] Implement the deterministic machine with no goroutines, clock reads, network I/O or persistence. Mode orders are auto = DE/NL/DIRECT, prefer-de = DE/NL/DIRECT, prefer-nl = NL/DE/DIRECT, direct = DIRECT. Every mode remains fail-open.
- [ ] Run:

        go test ./internal/failover -count=100
        go test -race ./internal/failover

  Expected: PASS with identical decisions on every repetition.
- [ ] Commit:

        git add internal/model internal/failover
        git commit -m "feat: add strict VLESS failover state machine"

## Task 3: Add a bounded loopback Clash API client

**Files:**
- Create: **internal/clash/client_test.go**
- Create: **internal/clash/client.go**
- Create: **internal/clash/types.go**

- [ ] Write httptest-based tests for:

        type Client interface {
            Delay(ctx context.Context, outbound, target string, timeout time.Duration) (DelayResult, error)
            Selected(ctx context.Context, selector string) (string, error)
            Select(ctx context.Context, selector, outbound string) error
            Traffic(ctx context.Context) (TrafficSnapshot, error)
            Connections(ctx context.Context) (ConnectionSnapshot, error)
            Version(ctx context.Context) (string, error)
        }
        func New(baseURL, secret string, httpClient *http.Client) (Client, error)

- [ ] Tests must verify escaped proxy names, Authorization: Bearer header, GET delay/selector/traffic/connections/version, PUT selector body, context cancellation, response size limits, non-2xx mapping and that neither secret nor raw response bodies appear in returned errors.
- [ ] Run:

        go test ./internal/clash -count=1

  Expected: FAIL because Client/New do not exist.
- [ ] Implement only the sing-box Clash endpoints required by gatewayd. Reject non-loopback baseURL during config validation; the client itself accepts a URL so tests can use httptest.
- [ ] Run:

        go test -race ./internal/clash
        go vet ./internal/clash

  Expected: PASS.
- [ ] Commit:

        git add internal/clash
        git commit -m "feat: add secure Clash API client"

## Task 4: Build probe orchestration, Selector reconciliation and state persistence

**Files:**
- Create: **internal/controller/service_test.go**
- Create: **internal/controller/service.go**
- Create: **internal/state/store_test.go**
- Create: **internal/state/store.go**
- Create: **internal/testutil/clock.go**

- [ ] Write controller tests around explicit ports:

        type Prober interface {
            Delay(context.Context, string, string, time.Duration) (clash.DelayResult, error)
        }
        type Selector interface {
            Selected(context.Context, string) (string, error)
            Select(context.Context, string, string) error
        }
        type SnapshotSink interface {
            Save(context.Context, model.StatusSnapshot) error
        }
        func New(Config, Prober, Selector, *failover.Machine, SnapshotSink, Clock) (*Service, error)
        func (s *Service) PollOnce(context.Context) (model.StatusSnapshot, error)
        func (s *Service) Run(context.Context) error

- [ ] Test that each 15-second cycle probes DE and NL against both configured URLs concurrently, but calls Observe once only after all four bounded results arrive. A timeout is a target failure; a cancelled service is not recorded as a route failure.
- [ ] Test Selector reconciliation: call Select only when desired differs from confirmed selection; failed Select reports control_error and preserves the last confirmed route; gatewayd shutdown never writes DIRECT or resets Selector.
- [ ] Write state-store tests for atomic temp-write/fsync/rename, mode persistence, bounded route history, UTC timestamps, corrupt-file quarantine and 0600 permissions.
- [ ] Run:

        go test ./internal/controller ./internal/state -count=1

  Expected: FAIL because the services are missing.
- [ ] Implement Service and Store. StatusSnapshot is the sole read model consumed by API and local CLI; no secret field is allowed in it.
- [ ] Run:

        go test -race ./internal/controller ./internal/state
        go test ./internal/... -count=1

  Expected: PASS.
- [ ] Commit:

        git add internal/controller internal/state internal/testutil
        git commit -m "feat: orchestrate probes and selector state"

## Task 5: Load validated runtime configuration and render sing-box atomically

**Files:**
- Create: **internal/config/types.go**
- Create: **internal/config/load_test.go**
- Create: **internal/config/load.go**
- Create: **internal/config/validate.go**
- Create: **internal/singbox/document.go**
- Create: **internal/singbox/render_test.go**
- Create: **internal/singbox/render.go**
- Create: **internal/singbox/check.go**
- Create: **internal/singbox/activate_test.go**
- Create: **internal/singbox/activate.go**
- Create: **config/examples/settings.example.json**
- Create: **config/examples/hardware.example.json**
- Create: **config/examples/secrets.example.json**
- Create: **config/schema/settings.schema.json**
- Create: **config/schema/hardware.schema.json**
- Create: **config/schema/secrets.schema.json**
- Create: **tests/fixtures/config/**

- [ ] Write loader tests for Load(root string) (Runtime, error). Runtime contains Settings, Hardware and Secrets loaded separately from settings.json, hardware.json and secrets.json. Unknown fields are errors. Error text contains a field path but never a rejected value from Secrets.
- [ ] Define exact roles upstream_2_5g, downstream_2_5g and management_1g. Validate unique normalized MACs, distinct interfaces, complete roles, transit/management/existing network non-overlap, management without default route, loopback Clash URL, two distinct VLESS profiles, exactly two probe URLs and thresholds 2/3/8 by default.
- [ ] Write render tests before implementation. Render(Runtime) must produce typed JSON, never textual substitution, with TUN interface nikas-tun0, IPv4 only, auto_route + auto_redirect, selector vless-active, outbounds vless-de/vless-nl/direct, rule order infrastructure → force-direct → force-vless → geosite-ru → geoip-ru → selected VLESS, loopback Clash API, DNS listener 127.0.0.1:5353 and DIRECT DNS behavior in fail-open.
- [ ] Pin reserved routing resources in tests: iproute2 table 20220, rule range 9020–9039, fallback rule 32760, input mark 0x2023, output mark 0x2024, nft table inet sing-box. Installer must later reject collisions.
- [ ] Run:

        go test ./internal/config ./internal/singbox -count=1

  Expected: FAIL because Load/Render/Activate are absent.
- [ ] Implement Loader and typed sing-box Document. Add:

        type Checker interface {
            Check(context.Context, string) error
        }
        type Reloader interface {
            Reload(context.Context) error
            Healthy(context.Context) error
        }
        func Activate(ctx context.Context, target string, data []byte, checker Checker, reloader Reloader) error

  Activate writes a 0600 temporary file in the target directory, fsyncs, checks it, renames atomically, reloads, verifies health and restores the previous generation on any post-rename failure.
- [ ] Add secret-canary tests to rendered-error, checker-error and rollback paths. Example secrets use unmistakable values beginning REPLACE_WITH_ and documentation-only IP ranges; no plausible production value.
- [ ] Run:

        go test -race ./internal/config ./internal/singbox
        go test ./internal/... -count=1

  Expected: PASS; golden config is deterministic.
- [ ] Commit:

        git add internal/config internal/singbox config tests/fixtures/config
        git commit -m "feat: render and activate validated sing-box config"

## Task 6: Implement staged RU rule-set updates with last-known-good rollback

**Files:**
- Create: **internal/rules/updater_test.go**
- Create: **internal/rules/updater.go**
- Create: **internal/rules/manifest.go**
- Create: **config/examples/force-direct.example.json**
- Create: **config/examples/force-vless.example.json**
- Create: **tests/fixtures/rules/**

- [ ] Write tests for:

        type Fetcher interface {
            Fetch(context.Context, string, io.Writer) (FetchMeta, error)
        }
        type Tool interface {
            ValidateBinary(context.Context, string) error
            CompileSource(context.Context, string, string) error
            CheckConfig(context.Context, string) error
        }
        type Activator interface {
            ReloadAndCheck(context.Context) error
        }
        func (u *Updater) Update(context.Context) (Manifest, error)

- [ ] Default sources are exactly:
  - https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-ru.srs
  - https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-ru.srs

  Permit source override only in non-secret settings. Enforce HTTPS, allow-list host, maximum size, non-empty body and SHA-256.
- [ ] Test staged generation creation, binary validation, local override compilation, complete config check, atomic current symlink switch, reload health check and manifest fields name/source_url/sha256/bytes/fetched_at/generation.
- [ ] Add failure tests for truncated download, wrong content, compiler failure, complete-config failure, interrupted staging, reload failure and corrupt current manifest. No failure may change the prior current symlink or delete its generation.
- [ ] Run:

        go test ./internal/rules -count=1

  Expected: FAIL because Updater is missing.
- [ ] Implement immutable generations under rules/generations/<UTC-id> and an atomically replaced relative rules/current symlink. Keep current plus two previous generations; garbage collection runs only after a successful health check.
- [ ] Run:

        go test -race ./internal/rules
        go test ./internal/... -count=1

  Expected: PASS.
- [ ] Commit:

        git add internal/rules config/examples tests/fixtures/rules
        git commit -m "feat: add atomic RU rule-set updater"

## Task 7: Expose authenticated, factual, read-only API v1

**Files:**
- Create: **internal/diagnostics/redact_test.go**
- Create: **internal/diagnostics/redact.go**
- Create: **internal/system/collector_test.go**
- Create: **internal/system/collector.go**
- Create: **internal/api/dto.go**
- Create: **internal/api/auth.go**
- Create: **internal/api/server_test.go**
- Create: **internal/api/server.go**
- Create: **config/schema/api-v1-status.schema.json**
- Create: **config/schema/api-v1-routes.schema.json**
- Create: **config/schema/api-v1-traffic.schema.json**
- Create: **config/schema/api-v1-system.schema.json**
- Create: **config/schema/api-v1-diagnostics.schema.json**
- Create: **config/schema/api-v1-health.schema.json**
- Create: **tests/contract/api-v1/**

- [ ] Write redaction tests first. Redactor uses an allow-list DTO, masks IPv4/IPv6/host/UUID/token/key-looking strings that reach free-text errors, preserves value type and never serializes Runtime.Secrets.
- [ ] Write Collector tests with an injected filesystem/command reader. Temperature, load, uptime, storage and component versions each return available/value/error; missing kernel files or command failures never become a factual zero.
- [ ] Write httptest cases for NewServer:

        type Provider interface {
            Snapshot(context.Context) model.StatusSnapshot
            Traffic(context.Context) model.TrafficSnapshot
            System(context.Context) model.SystemSnapshot
            Diagnostics(context.Context) diagnostics.Report
            Health(context.Context) model.ComponentHealth
        }
        type ServerConfig struct {
            ManagementListen string
            BearerToken       string
            LoopbackLiveness  bool
        }
        func NewServer(ServerConfig, Provider, buildinfo.Info) (http.Handler, error)

- [ ] Freeze GET routes /api/v1/status, /routes, /traffic, /system, /diagnostics and /health. Every body includes schema_version, gateway_version, generated_at UTC, stable state and freshness. Add optional /livez without auth only when RemoteAddr is loopback.
- [ ] Test valid bearer, missing/wrong bearer 401, unknown path 404, POST/PUT/PATCH/DELETE 405, oversized header rejection, no permissive CORS, content-type, no-cache, request ID, panic recovery with redacted log and no response secret.
- [ ] Add contract fixtures covering healthy Germany, Netherlands fallback, degraded_fail_open, stale data and unavailable metric. Validate fixtures against JSON Schemas.
- [ ] Run:

        go test ./internal/diagnostics ./internal/system ./internal/api -count=1

  Expected: FAIL because the handlers and collectors are missing.
- [ ] Implement fixed-length SHA-256 digest comparison with crypto/subtle for bearer authentication, bounded JSON encoding and graceful HTTP shutdown. Do not add any write endpoint.
- [ ] Run:

        go test -race ./internal/diagnostics ./internal/system ./internal/api
        python3 -m check_jsonschema --schemafile config/schema/api-v1-status.schema.json tests/contract/api-v1/status-*.json

  Expected: PASS.
- [ ] Commit:

        git add internal/diagnostics internal/system internal/api config/schema tests/contract
        git commit -m "feat: publish read-only gateway API v1"

## Task 8: Wire gatewayd, gatewayctl and the root-only local control socket

**Files:**
- Create: **internal/control/protocol.go**
- Create: **internal/control/server_test.go**
- Create: **internal/control/server.go**
- Create: **internal/control/client.go**
- Create: **internal/app/gatewayd_test.go**
- Create: **internal/app/gatewayd.go**
- Create: **internal/app/gatewayctl_test.go**
- Create: **internal/app/gatewayctl.go**
- Create: **cmd/gatewayd/main.go**
- Create: **cmd/gatewayctl/main.go**

- [ ] Write protocol tests for a length-bounded one-request/one-response JSON message over /var/run/nikas-vless/gatewayd.sock. Supported local operations: status, diagnostics, validate, reload, rules-update and set-mode with auto/prefer-de/prefer-nl/direct. Unknown fields/commands fail closed.
- [ ] Test socket ownership/mode 0600, stale socket removal only after confirming it is a socket, request deadline, peer disconnect and that no listener is created on TCP.
- [ ] Write app tests with fake config, controller, rules updater, API listener and signal source. Startup order is load → validate → connect Clash → load state → controller → read-only API → control socket. Shutdown stops listeners/controller but never resets Selector.
- [ ] Write gatewayctl parsing/exit-code tests. Read-only commands can run as the device user allowed by the socket ACL; every mutating command requires effective UID 0 and an explicit subcommand. restore additionally requires --backup and --yes.
- [ ] Run:

        go test ./internal/control ./internal/app -count=1

  Expected: FAIL because control/app packages do not exist.
- [ ] Implement thin main packages with --root, --config-root, --version and structured redacted logging. No business logic in cmd/.
- [ ] Run:

        go test -race ./internal/control ./internal/app
        make build-arm64
        GOOS=linux GOARCH=arm64 go version -m dist/gatewayd
        GOOS=linux GOARCH=arm64 go version -m dist/gatewayctl

  Expected: PASS; both binaries contain 1.0.0-beta001 and no VLESS data.
- [ ] Commit:

        git add internal/control internal/app cmd
        git commit -m "feat: wire gateway daemon and local CLI"

## Task 9: Add FriendlyWrt/procd, fail-open interception cleanup and DNS fallback

**Files:**
- Create: **openwrt/etc/config/nikas-vless**
- Create: **openwrt/etc/init.d/nikas-sing-box**
- Create: **openwrt/etc/init.d/nikas-gatewayd**
- Create: **openwrt/usr/libexec/nikas-sing-box-run**
- Create: **openwrt/usr/libexec/nikas-interception-cleanup**
- Create: **openwrt/etc/nikas-vless/templates/90-nikas-vless.nft.tmpl**
- Create: **openwrt/etc/nikas-vless/templates/dnsmasq.conf.tmpl**
- Create: **openwrt/etc/nikas-vless/templates/sysctl.conf.tmpl**
- Create: **openwrt/etc/nikas-vless/cron.fragment**
- Create: **openwrt/etc/hotplug.d/iface/90-nikas-vless**
- Create: **tests/openwrt/check_overlay.sh**
- Create: **tests/openwrt/test_fail_open.sh**
- Create: **tests/openwrt/fixtures/**

- [ ] Write shell tests using a temporary NIKAS_ROOT and PATH command shims. Assert procd respawn/file/netdev triggers, config check before start, gatewayd dependency on config but not vice versa, dnsmasq strict-order 127.0.0.1#5353 then Keenetic resolver, IPv4 forwarding and explicit IPv6 transit disable.
- [ ] Add a fixture representing stale table inet sing-box plus reserved ip rules/routes. Test cleanup deletes only inet sing-box, priorities 9020–9039, fallback 32760 and table 20220; unrelated fw4/UCI/routes remain byte-for-byte unchanged.
- [ ] Add crash tests: wrapper pre-cleans, launches foreground sing-box, traps child exit, invokes cleanup and exits nonzero for procd respawn. If the child is SIGKILLed, the wrapper must still clean. If the wrapper is SIGKILLed, its next procd start must pre-clean before launching.
- [ ] Test nft guard template: management cannot forward transit traffic; API input is limited to configured HA host/admin CIDR; downstream IPv6 forward is rejected; IPv4 upstream↔downstream forwarding remains permitted; no masquerade, snat, dnat or redirect is present in project-owned guard rules.
- [ ] Run:

        sh tests/openwrt/check_overlay.sh
        sh tests/openwrt/test_fail_open.sh

  Expected: FAIL until overlay scripts/templates exist.
- [ ] Implement POSIX/BusyBox-compatible scripts. sing-box owns auto_route/auto_redirect; the wrapper only validates and removes the exact reserved artifacts after process loss. Render firewall include into /usr/share/nftables.d/ruleset-post/90-nikas-vless.nft; do not hard-code Linux interface names.
- [ ] Ensure gatewayd procd stop never stops nikas-sing-box. sing-box stop/restart leaves dnsmasq running, so its second strict-order Keenetic resolver provides DNS and status becomes degraded_fail_open after gatewayd observes local DNS failure.
- [ ] Run:

        shellcheck openwrt/etc/init.d/* openwrt/usr/libexec/* openwrt/etc/hotplug.d/iface/* tests/openwrt/*.sh
        sh tests/openwrt/check_overlay.sh
        sh tests/openwrt/test_fail_open.sh

  Expected: PASS.
- [ ] Commit:

        git add openwrt tests/openwrt
        git commit -m "feat: add FriendlyWrt fail-open service overlay"

## Task 10: Implement idempotent install, inventory, backup, restore and doctor flows

**Files:**
- Create: **scripts/lib/common.sh**
- Create: **scripts/preflight.sh**
- Create: **scripts/install.sh**
- Create: **scripts/backup.sh**
- Create: **scripts/restore.sh**
- Create: **scripts/doctor.sh**
- Create: **tests/install/test_preflight.sh**
- Create: **tests/install/test_install.sh**
- Create: **tests/install/test_backup_restore.sh**
- Create: **tests/install/fixtures/**

- [ ] Write preflight tests before scripts. Mock board info, ip/route output, UCI and MAC inventory. Accept only FriendlyWrt 24.10.2 + NanoPi R5S + aarch64 + sing-box 1.12.22 with Clash API. Reject missing packages, duplicate MAC, missing link role, interface-name drift, transit overlap with any supplied/current route, management default route and reserved nft/ip-rule collision.
- [ ] Assert every preflight rejection occurs before the first call to backup, uci set/commit, service stop or file replacement. Use an append-only command journal fixture to prove order.
- [ ] Write install tests for --stage direct and --stage policy. Default invocation installs binaries/config/examples/overlay but leaves transit policy DIRECT. --stage policy is refused until direct-stage health evidence exists in local state.
- [ ] Test idempotence, root requirement, atomic file replacement, correct 0755/0644/0600 modes, no overwrite of existing secrets.json, placeholder-secret refusal, no eMMC formatting/reflash command and rollback on any post-backup error. On first install, accept VLESS material only through --secrets-file (never CLI value flags); if API/Clash tokens are absent, generate two independent random tokens from the kernel CSPRNG and write the completed file once with mode 0600.
- [ ] Write backup/restore tests. Backup contains UCI network/firewall/dhcp, installed files, package/version inventory, runtime config, current rule generation and secret file only inside a root-owned 0600 archive; manifest records SHA-256. Restore verifies checksum/model/version, stops services in safe order, restores atomically and returns DIRECT if policy health fails.
- [ ] Write doctor tests for hardware roles, routes, no NAT, IPv6 disabled, dnsmasq upstream order, sing-box/gatewayd health, API bind, rule freshness and disk/temperature. Output is redacted and has human plus --json modes.
- [ ] Run:

        sh tests/install/test_preflight.sh
        sh tests/install/test_install.sh
        sh tests/install/test_backup_restore.sh

  Expected: FAIL until scripts exist.
- [ ] Implement scripts with NIKAS_ROOT and command shims for tests, but fixed production paths by default. Print the exact restore command immediately after successful backup.
- [ ] Run:

        shellcheck scripts/*.sh scripts/lib/*.sh tests/install/*.sh
        sh tests/install/test_preflight.sh
        sh tests/install/test_install.sh
        sh tests/install/test_backup_restore.sh

  Expected: PASS.
- [ ] Commit:

        git add scripts tests/install
        git commit -m "feat: add safe NanoPi installer and rollback"

## Task 11: Produce a deterministic ARM64 artifact and complete CI gates

**Files:**
- Create: **tools/versions.env**
- Create: **tools/checksums/sing-box-1.12.22.sha256**
- Create: **scripts/fetch-sing-box-checker.sh**
- Create: **scripts/scan-secrets.sh**
- Create: **scripts/package.sh**
- Create: **tests/package/check_archive.sh**
- Modify: **.github/workflows/repository-checks.yml**
- Modify: **.gitignore**

- [ ] Write package tests for exact archive name nikas-vless-gateway-1.0.0-beta001-aarch64.tar.gz, sorted paths, fixed SOURCE_DATE_EPOCH, uid/gid 0, executable modes, no workspace absolute paths, manifest SHA-256 and required contents.
- [ ] Write pin tests requiring GO_VERSION=1.27.2, SING_BOX_VERSION=1.12.22, a literal 64-hex official Linux ARM64 archive checksum, and literal SHA-256/source-commit pins for the packaged geosite-ru.srs and geoip-ru.srs snapshot. fetch-sing-box-checker.sh verifies the downloaded archive before extracting and verifies sing-box version plus Clash API config support.
- [ ] Write secret-scan fixtures with one value for every prohibited class. The scanner must fail on private fixture credentials, UUID, REALITY key, short ID, token and subscription URL, but allow explicit REPLACE_WITH_ placeholders and RFC 5737/3849 documentation addresses.
- [ ] Run:

        sh tests/package/check_archive.sh
        sh scripts/scan-secrets.sh --self-test

  Expected: FAIL until packaging and scanner exist.
- [ ] Implement package.sh to build static linux/arm64 gatewayd and gatewayctl, assemble overlay/examples/schemas plus the source-commit/SHA-pinned initial RU rule generation, create deterministic tar.gz and matching .sha256. Do not package a production secrets.json.
- [ ] Expand CI into explicit jobs: gofmt/vet; unit + race; JSON/Schema; pinned sing-box render/check; ShellCheck + shell tests; secret scan; ARM64 build; deterministic package twice and byte-compare; artifact inventory/checksum. Keep the existing repository safety checks.
- [ ] Obtain the literal sing-box checksum from the official SagerNet 1.12.22 GitHub release checksum asset during implementation, commit it to tools/checksums, and make CI reject missing, changed or unverified pins. Never use latest.
- [ ] Run:

        make verify
        sh scripts/package.sh
        sh tests/package/check_archive.sh dist/nikas-vless-gateway-1.0.0-beta001-aarch64.tar.gz

  Expected: PASS and a matching checksum file.
- [ ] Commit:

        git add tools scripts tests/package .github/workflows/repository-checks.yml .gitignore
        git commit -m "build: package and verify ARM64 gateway release"

## Task 12: Add operational docs, contract checks and release gates

**Files:**
- Create: **docs/API.md**
- Create: **docs/DEPLOYMENT.md**
- Create: **docs/ROLLBACK.md**
- Create: **docs/ACCEPTANCE.md**
- Create: **docs/OPERATIONS.md**
- Create: **tests/docs/check_runbooks.sh**
- Modify: **README.md**
- Modify: **CHANGELOG.md**
- Modify: **docs/RELEASES.md**

- [ ] Write check_runbooks.sh first. Require exact sections for supported hardware/OS, cable labels, MAC inventory, subnet overlap, backup path, generated rollback command, DIRECT-first gate, TverLine/LTE test, DE→NL→DIRECT failure tests, DNS fallback, management isolation, 24-hour soak, HA connection, physical bypass and evidence recording.
- [ ] Require every destructive or connectivity-changing command to be preceded by a verification command and followed by expected output plus rollback command. Runbooks must never contain product secrets, example UUIDs or real server addresses.
- [ ] Run:

        sh tests/docs/check_runbooks.sh

  Expected: FAIL while documents/README still describe the old skeleton.
- [ ] Write API.md from the schemas, including freshness/unavailable semantics and explicit absence of write endpoints. Write OPERATIONS.md for status, logs, manual modes, safe reload, weekly rules, storage and temperature.
- [ ] Write DEPLOYMENT.md as gated procedure: backup → bench → inline DIRECT → Keenetic failover → routing policy → failover → recovery → 24-hour soak → HA. Write ROLLBACK.md for software restore and labeled Keenetic-to-UXG-Max physical bypass.
- [ ] Update README to NanoPi R5S-only and remove Raspberry Pi/systemd references. Add unreleased 1.0.0-beta001 notes, artifact/evidence rules and “no release tag before bench acceptance” to release docs.
- [ ] Run:

        sh tests/docs/check_runbooks.sh
        make verify

  Expected: PASS.
- [ ] Commit:

        git add README.md CHANGELOG.md docs tests/docs
        git commit -m "docs: add NanoPi deployment and acceptance runbooks"

## Task 13: Connect the separate Home Assistant integration read-only

**Repository:** NikaSir/ha-vless-gateway, new branch feature/read-only-gateway-api-v1. Begin only after Tasks 7 and 11 freeze passing API fixtures.

**Files:**
- Create: **custom_components/vless_gateway/api.py**
- Create: **custom_components/vless_gateway/coordinator.py**
- Create: **custom_components/vless_gateway/sensor.py**
- Create: **custom_components/vless_gateway/binary_sensor.py**
- Modify: **custom_components/vless_gateway/config_flow.py**
- Modify: **custom_components/vless_gateway/const.py**
- Modify: **custom_components/vless_gateway/__init__.py**
- Modify: **custom_components/vless_gateway/manifest.json**
- Modify: **custom_components/vless_gateway/panel_manifest.json**
- Modify: **custom_components/vless_gateway/frontend/src/vless-gateway-panel.js**
- Regenerate: **custom_components/vless_gateway/frontend/vless-gateway-panel.js**
- Create: **tests/fixtures/api_v1/** copied from the released gateway contract
- Create: **tests/test_api_client.py**
- Create: **tests/test_coordinator.py**
- Create: **tests/test_entity_mapping.py**
- Create: **tests/test_config_flow.py**
- Modify: **README.md**
- Modify: **CHANGELOG.md**
- Modify: **docs/VLESS_GATEWAY_PANEL.md**
- Modify: **.github/workflows/repository-checks.yml**

- [ ] Create the branch from the current main head; record the exact vless-gateway commit containing the copied API fixtures so contract drift is reviewable.
- [ ] Write async client tests first for GatewayClient(base_url, token, session): HTTPS or explicitly trusted private-management HTTP only, bearer auth, bounded response, schema_version == 1, timeout/error mapping and no token in exceptions/logs.
- [ ] Write DataUpdateCoordinator tests: one coordinated refresh obtains status/routes/traffic/system/health, keeps last good values only with stale=true, marks unavailable accurately and uses diagnostics only on explicit diagnostic refresh.
- [ ] Write entity tests mapping the reserved panel roles: gateway/lan/tunnel/server/internet status; active/default/DNS route; latency; uptime; bypass count; last route change; rates/totals; active connections. available=false produces unavailable, never zero.
- [ ] Write config-flow tests for host/token validation, duplicate gateway rejection, auth failure, timeout, schema mismatch and token storage in the config entry rather than options or entity attributes.
- [ ] Run:

        python -m unittest discover -s tests -v
        node scripts/build-frontend-bundle.mjs --check

  Expected: FAIL before api/coordinator/platform implementation.
- [ ] Implement client, coordinator and sensor/binary_sensor platforms. The browser panel reads HA entities/coordinator data only; it never calls NanoPi directly and contains no token. Keep every route/mode/reload control disabled and remove any UI affordance that implies a write command.
- [ ] Update manifests/docs, rebuild the checked-in frontend bundle with the existing script and verify no API token is emitted into the bundle.
- [ ] Run:

        python -m compileall -q custom_components tests
        python -m unittest discover -s tests -v
        node scripts/build-frontend-bundle.mjs --check
        node --check custom_components/vless_gateway/frontend/vless-gateway-panel.js
        node tests/shell_v2_harness.mjs
        python scripts/check_nikas_ui_standard.py
        python scripts/check_standard_documents.py

  Expected: PASS; HACS and Hassfest must also pass in GitHub Actions.
- [ ] Commit in ha-vless-gateway:

        git add custom_components tests README.md CHANGELOG.md docs .github/workflows/repository-checks.yml
        git commit -m "feat: consume gateway API v1 read-only"

## Final verification and deployment gate

- [ ] In vless-gateway, run make verify and produce the archive twice from clean checkouts; SHA-256 values must match.
- [ ] Inspect git diff from the approved spec commit and confirm there is no systemd/Raspberry/IPv6-transit/NAT/write-API scope creep.
- [ ] Run a repository-wide prohibited-secret scan on tracked files and the unpacked archive.
- [ ] Review all five Review Focus items against their named tests; attach command outputs to the implementation PR.
- [ ] Do not create a release tag and do not touch the live NanoPi until the owner approves the implementation PR and the bench execution window.
- [ ] Bench deployment follows docs/ACCEPTANCE.md and stops on the first failed gate. Inline policy enablement and the 24-hour soak are separate explicit approvals.
