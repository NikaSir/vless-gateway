# NanoPi R5S implementation and review record

Candidate: **1.0.0-beta001**, API schema **1**. Scope: NanoPi R5S LTS, FriendlyWrt 24.10.2, sing-box 1.12.22; separate read-only Home Assistant integration.

The 13 approved implementation tasks are complete. This is a bench candidate, not evidence of a working inline deployment. No device installation, production credential access, merge, release tag or live HA change was performed.

## Independent review and disposition

One independent whole-branch review covered gateway `aa4a0a9..3717935` and HA `532d45f..c54b72b`. It identified no Critical issues, seven Important issues and one Minor issue. All eight were accepted and addressed in one repair pass. No second independent review was requested; the fixes were verified with regression tests and the required CI gates.

The state-severity issue was regraded Important by user effect: a real controller failure appeared active in the panel. There are no deferred minors.

| Finding | Final behavior and evidence |
|---|---|
| Pinned sing-box health rejected the real version response | Health accepts the exact upstream `sing-box 1.12.22` representation. The HTTP regression failed with a deadline before the fix and passes afterwards. The Clash mock now matches [tagged upstream source](https://github.com/SagerNet/sing-box/blob/v1.12.22/experimental/clashapi/server.go). |
| First-install restore left new guard, sysctl and service files | Backup format 2 records the owned path scope; archive members record presence and contents. Restore reconciles absence and contents, including service enablement links, while preserving backup archives and unrelated files. First-install/upgrade regressions failed before the fix and pass afterwards. Destination symlink ancestors are rejected before writes. |
| Installer signals and late failures bypassed rollback | Every post-backup failure and catchable signal reaches the EXIT rollback handler. A synced journal permits recovery after SIGKILL/power interruption on the next installer invocation. Tests inject render failure, topology publication failure, final version write failure, TERM and SIGKILL. All 13 UCI/service operations propagate failure to the installer. Restore reports service recovery errors after attempting all recovery operations. |
| Fresh polling reused a route after Selector loss | Unobserved or unconfirmed selection becomes unavailable; historical route changes remain separate. Regression: healthy DE, Selector loss and continued polling. Public API remains fresh but route availability is false; HA maps it to unavailable. DNS fallback is independently probed using the configured DIRECT resolver. |
| Interrupted config publication overwrote known-good bytes on retry | A synced transaction marker protects `.previous` until runtime acknowledgement. Startup and retry recover unfinished publication first. Tests interrupt after publication and before health acknowledgement, then retry; known-good bytes survive. Normal startup recovers, while the explicit in-transaction restart checks the candidate. |
| Fail-open test only asserted command strings | Required CI now executes real `nft`/`ip` state cleanup in a network namespace and real dnsmasq queries against distinguishable primary/fallback resolvers, before and after SIGKILL of the wrapped child. It checks unrelated kernel state survives and dnsmasq stays reachable. The former shim test remains a quick lifecycle check. |
| HA presented all sing-box bytes as protected VLESS traffic | The sensor and panel say “Трафик sing-box” and explain DIRECT + VLESS coverage and exclusion of kernel bypass. A DIRECT fixture and rendered-panel harness verify the description. No unsupported per-outbound accounting is invented. |
| Minor: failure states used normal tone/raw codes | `degraded_fail_open`, `control_error` and `starting` now have Russian labels and warning/error tones, tested through the panel methods. |

## Verification and limits

Gateway validation includes formatting, vet, unit/race tests, real pinned sing-box configuration validation, JSON contracts, ShellCheck, installer/restore/lifecycle tests, secret scanning and reproducible ARM64 packaging. HA validation includes 32 unit/contract tests and four tests running actual HA coordinator, flows, setup and entity registry, plus the generated frontend bundle checks. GitHub CI additionally requires unskipped root Unix-socket and isolated kernel/DNS tests.

This execution environment rejects AF_UNIX creation and network namespaces. The local Unix-socket skip is explicit; neither restriction is allowed to skip its required GitHub CI gate. Tests and CI establish only their stated scope.

The reviewer declined to judge the following, and these limits remain explicit:

- Actual NanoPi throughput, physical port identity, FriendlyWrt kernel behavior, inline TverLine/LTE failover and 24-hour stability require device/inline/soak evidence.
- Live sing-box startup in the review environment returned `operation not permitted`; no runtime success there was claimed. The version finding used the exact tagged source.
- Automatic cron installation before acceptance was not a defect: OPERATIONS requires manually installing the supplied fragment after bench acceptance while retaining other entries.
- Unmeasured LAN, internet and period-accounting values remain explicitly unavailable, rather than fabricated.
- Other hardware, IPv6 transit, public write APIs and fail-open with an unpowered device remain outside support scope.

The remaining deployment gate is the acceptance sequence in ACCEPTANCE.md: backup/restore, bench DIRECT, inline DIRECT, policy/failure tests, then 24-hour soak and owner release approval.

## Implementation decisions

These are the execution ledger rulings, including the reason and cost if wrong. No review finding was deferred.

1. Fresh clone on a feature branch supplies isolation; no extra linked worktree is needed because no prior user checkout exists here. Cost: an extra local clone.
2. Skill helper scripts were read from the installed plugin cache after resource URI lookup failed. Same installed skill; no product impact.
3. Human runbooks receive manual review instead of grep tests, following current test guidance. Cost: a documentation omission might escape review.
4. gatewayd never reads VLESS material: API/Clash authentication is separate from renderer-only profiles. Reason: binding credential boundary. Cost: configuration schema migration.
5. Preserve a confirmed existing Selector during controller bootstrap. Reason: controller restart must retain the data plane. Cost: fail-open can wait up to the configured failure threshold (three cycles by default); if selection is no longer observable, current route is unavailable.
6. Use pinned jsonschema 4.25.1 through check_contracts.py instead of the missing check_jsonschema CLI. Same JSON Schema validation; no relevant CLI parity cost.
7. Permit an explicit local AF_UNIX test skip only in this restricted environment. Required CI runs the real root socket test without the skip.
8. Pin official GitHub release asset digests because sing-box 1.12.22 has no checksum text asset. Cost: a wrong pin rejects the download instead of executing unverified bytes.
9. Typed Go backup/restore and semantic preflight sit behind thin ash entrypoints. Reason: safe archive/path and CIDR validation. Cost: packaged gatewayctl is required.
10. Installation prepares topology by default; `--apply-network` applies it. Reason: make exact network files reviewable before a change. Cost: one deployment flag.
11. Topology application requires a non-empty Dropbear authorized_keys file before disabling passwords. Cost: deployment waits for SSH key provisioning; actual SSH login remains a bench check.
12. Use official geosite-category-ru.srs under the local geosite-ru tag because upstream geosite-ru.srs is absent. Cost: RU classification coverage still needs bench checks and local overrides.
13. Add gateway-checks while preserving existing repository checks. Cost: branch protection may need the additional gate selected.
14. Pin HA tests to Python 3.14, HA 2026.10.0, pytest-homeassistant-custom-component 0.13.371 and frontend 20260930.2. Reason: real supported HA validation. Cost: newer development runtime.
15. Extend the local UI version validator to betaNNN without changing pinned canonical docs. Reason: approved full beta version requirement. Cost: central checker policy needs follow-up alignment.
16. Preserve legacy empty scaffold entries and use Reconfigure to add connection details. Cost: no telemetry until they are configured.
17. Publish via the configured GitHub Git-data connector after native git push lacked credentials. Exact source trees and task commit messages were preserved; commit SHAs differ because authoring metadata differs.
18. Use backup format 2 for authoritative restore scope. Reason: pre-install absence must be recoverable. Cost: format-1 development archives are rejected; no released/deployed format-1 archive was part of this work.
19. A live activation restart explicitly defers boot recovery until health acknowledgement; ordinary startup always recovers an unfinished transaction. Reason: boot recovery must not undo a candidate during its own verification. Cost: root-only service orchestration carries one internal environment flag.
20. Rename the measured HA traffic total rather than invent VLESS-only accounting. Cost: protected-traffic accounting remains unavailable until supported by a separate measured contract.

No merge, tag, production deployment or remote message to another person is authorized by this record.


## Beta001 follow-up review repairs — 2026-10-20

The next independent review found five further defects. The local repair keeps candidate/API versions and default thresholds unchanged; no device or live HA acceptance is claimed.

| Finding | Repair and behavioral evidence |
|---|---|
| Recovery re-entered bootstrap and retained a failed VLESS route while the other path flapped | Bootstrap preservation is latched to the initial phase. Readiness honors the configured selected-path failure threshold. DE/NL regressions cover failure thresholds 2, 3 and 5 and controller-only restart. |
| Ordinary startup ignored an unfinished rule transaction; rollback discarded evidence despite failure | Root-only `recover-rules` restores disk state before startup validation, under the updater lock. Transaction restarts preserve their candidate. Interrupted-publication, failed reload/switch, corrupt journal and startup-order tests check retained evidence and factual errors. |
| Persistent selector cache could override cold-start DIRECT | Generated configuration disables the unnecessary cache. Structural regression failed against the former configuration. A pinned 1.12.22 restart characterization seeds each DE/NL cached choice, verifies persistence, then requires generated settings to start DIRECT. |
| Tunnel telemetry equated successful API control with a working VLESS path | Confirmed DIRECT reports false; selected VLESS reports its measured path health; unconfirmed selection or unmeasured health is unavailable. Gateway-produced snapshots are shared unchanged with HA role and rendered-panel tests. |
| Desired changes overwrote confirmed last-change metadata | Polling/mode decisions preserve confirmed LastChange. Restore retains confirmed history without restoring current observations. Missing acknowledgement keeps prior history; later acknowledgement records one change at observation time, with repeated polls adding no duplicates. |

Local verification: `make verify`, pinned configuration validation, JSON contracts, ShellCheck, shell lifecycle/preflight/backup/installer, secret scan, HA unittest/pytest, deterministic frontend and pinned standards checks. HA consumes corrected source observations without production mapping/UI changes.

Local limitations remain explicit: AF_UNIX and sing-box runtime startup return EPERM, and network namespace creation is denied. The explicit `NIKAS_ALLOW_SOCKET_SKIP=1` only permits these local socket/runtime cases; required GitHub CI does not set it. Real pinned selector restarts and root socket/kernel/DNS gates must pass in CI. Repository checks do not establish NanoPi, FriendlyWrt or inline/soak acceptance.
