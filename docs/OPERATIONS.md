# Operations

gatewayctl status/diagnostics read the local root-only socket. gatewayctl doctor returns redacted component diagnostics. Use authenticated /api/v1/system for uptime/load/temperature/storage; unavailable is not a healthy zero.

Root can request gatewayctl set-mode auto, prefer-de, prefer-nl or direct. Each remains fail-open. Read the confirmed route and error state; an unacknowledged request is not a completed change.

Before editing config, retain gatewayctl backup, edit local 0600 inputs and run gatewayctl validate. Then gatewayctl reload. Failed validation is rejected; failed activation restores the previous generation. Stopping gatewayd does not stop sing-box.

gatewayctl rules-update performs a staged manual refresh. The provided cron fragment is Sunday 04:17 device time; install it into the existing root crontab only after bench acceptance, preserving other entries. Binary parsing, complete config check and reload health precede success; two previous generations remain.

Overrides live at /etc/nikas-vless/overrides/force-direct.json and force-vless.json in source rule-set format. Apply with rules-update. Precedence is infrastructure, force-direct, force-vless, Russian domain/IP sets, selected VLESS default.

geosite-ru is the local tag for SagerNet's geosite-category-ru.srs; upstream publishes no geosite-ru.srs. Packaged snapshots have source-commit/SHA pins in tools/rules.lock.json. Weekly refresh uses official rule-set branches and retains a manifest.

logread -e nikas-vless provides safe lifecycle events. Raw sing-box errors are discarded because they can contain endpoints. Reproduce detailed validation locally with controlled output when needed; never share unredacted output.

settings/hardware are separate from secrets.json (profiles) and auth.json (API/Clash tokens). gatewayd reads only settings/hardware/auth; root CLI subprocesses handle profile rendering. All runtime credential files remain root-owned 0600 and outside Git.

Investigate missing temperature/storage metrics rather than treating them as healthy. Remove only identified obsolete backups after retaining a tested restore point. See ROLLBACK.md for physical bypass.
