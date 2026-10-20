# Acceptance record — 1.0.0-beta001

Record archive SHA-256, exact versions, UTC start/end, operator, physical MAC roles, expected/actual result and pass/fail/untested for every check. Keep endpoint values and credentials out of shared evidence. Repository checks do not prove FriendlyWrt kernel/service behavior.

## Bench gate

- FriendlyWrt backup/package inventory retained; restore performed on the same NanoPi.
- All three ports identified by link/MAC test; transit subnet has no overlap.
- Keenetic reservation/return route and original UXG-Max WAN profile retained.
- DIRECT, Germany and Netherlands each establish new HTTPS sessions with expected egress.
- Russian/non-Russian samples show expected split DNS and egress; DNS success alone is not routing evidence.
- Management has no default route/forwarding; administrator SSH keys and API restrictions work.
- Stopping/restarting gatewayd retains the confirmed Selector until new probes qualify a change.
- Stop and SIGKILL sing-box: interception disappears; new DIRECT sessions and Keenetic-side DNS work. Kill/restart the wrapper separately.
- Invalid config, malformed rules, truncated download and interrupted publication retain known-good data.
- Physical bypass has actually succeeded with the original UXG-Max WAN profile.

Every bench entry must pass before inline installation.

## Inline DIRECT gate

- No transit masquerade/SNAT rules; new internet/DNS sessions work with policy disabled.
- Disconnect TverLine: Keenetic LTE preserves new sessions; restore TverLine and verify return.
- Management isolation, SSH, HA API source restriction and loopback-only Clash API verified.
- DNS fallback remains available when sing-box stops.
- Current backup and exact rollback command verified.

Retain accepted DIRECT evidence at /etc/nikas-vless/state/direct-accepted.json before policy installation. This is an operator acceptance record, not a flag to create before performing checks.

## Policy gate

- Cold startup remains DIRECT until two successful cycles qualify VLESS.
- Russian samples use provider egress; non-Russian use Germany.
- Three failed cycles on both Germany probe targets select healthy Netherlands.
- Both paths failing select DIRECT/degraded_fail_open; new sessions and DNS continue.
- Eight consecutive successful cycles restore priority: Germany first, otherwise recovered Netherlands.
- One failed target alone does not fail the route; flapping/stale cycles do not bypass hold-down.
- Local preference changes acknowledge the actual Selector; no black-hole mode exists.
- IPv6 transit is rejected; invalid config/rule refresh rolls back.

## 24-hour soak and HA

Observe at least 24 hours with regular new-session/DNS tests, route history, temperature, storage, restarts and provider changes. Any unexplained loss restarts the soak after correction.

Connect ha-vless-gateway using the separate API token. Verify six GET endpoints, role mapping, stale/unavailable handling, full integration/UI versions and disabled write controls. Confirm no token in browser assets/storage, entity attributes, logs or diagnostic exports.

Release requires actual device evidence, green CI, independent review and owner approval. No release tag before bench acceptance.
