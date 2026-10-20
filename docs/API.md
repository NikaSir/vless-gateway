# Gateway API v1

The service binds the configured management IPv4 address (default port 9080). Firewall access is limited to the HA host and administrator subnet. All six endpoints require `Authorization: Bearer <device API token>`; the independent Clash token is never used by HA.

| GET path | Data |
|---|---|
| `/api/v1/status` | Mode, desired/confirmed route, observations and history |
| `/api/v1/routes` | DE/NL probes, latency, counters and last route change |
| `/api/v1/traffic` | Rates, sing-box totals and connections; daily/monthly accounting may be unavailable |
| `/api/v1/system` | Uptime, load, temperature, storage and versions |
| `/api/v1/diagnostics` | Allow-listed redacted diagnostics |
| `/api/v1/health` | Controller, sing-box API and local DNS observations |

Every successful response contains `schema_version: "1"`, `gateway_version`, UTC `generated_at`, `state`, `freshness` and `data`. Schemas live in config/schema/api-v1-*.schema.json; shared fixtures in tests/contract/api-v1/.

Freshness is the age of the last complete probe cycle; more than 45 seconds is stale. Metrics use available/value/error: unavailable means `available: false`, `value: null`, not zero. Rates are bytes/second, totals bytes, latency milliseconds, uptime seconds and temperature Celsius. `desired_route` is the decision; `active_route` requires Selector acknowledgement. Fresh probes do not prove all components healthy. `tunnel_status` is false for confirmed DIRECT, uses the selected VLESS path’s measured health for DE/NL, and is unavailable when selection is unconfirmed or its health has not been measured. `last_route_change` and `history` describe acknowledged changes only; a failed request leaves them unchanged. A request timeout or cancellation can follow an applied mutation; history stays unchanged until a matching successful observation confirms it. Delayed acknowledgement uses its observation time, not an inferred switch time. Restoring controller state retains confirmed history but never treats saved route/health as a current observation.

Modes: auto/prefer-de/prefer-nl/direct. Routes: de/nl/direct. States: starting/healthy/degraded_fail_open/control_error. HTTP 401 means failed authentication, 404 unknown path, 405 forbidden method, 500 unavailable response. Errors are safe codes, never raw upstream messages.

No HTTP write route exists. Optional /livez is loopback-only and does not replace component health. Public exposure and permissive CORS are unsupported. Root operations use gatewayctl and the 0600 Unix socket /var/run/nikas-vless/gatewayd.sock.
