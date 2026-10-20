# NikaS VLESS Gateway

Gateway foundation **1.0.0-beta001** for NanoPi R5S LTS 4+64 GB, FriendlyWrt 24.10.2 and sing-box 1.12.22. Russian resources use DIRECT; other IPv4 traffic follows Germany → Netherlands → DIRECT. The 1 GbE port is management only; NanoPi adds no SNAT/masquerade.

## Scope boundary

This repository contains the gateway itself: host provisioning, network/service configuration, deployment logic, diagnostics, backup/restore procedures, and operational documentation.

The Home Assistant integration is a separate product and lives in **`NikaSir/ha-vless-gateway`**. Home Assistant-specific code must not be mixed into this repository.

## Target platform

Only NanoPi R5S is supported. This is a bench-validation candidate: real-device acceptance, physical bypass and a 24-hour soak are required before release/household deployment.

- [Deployment](docs/DEPLOYMENT.md)
- [Acceptance gates](docs/ACCEPTANCE.md)
- [Rollback and bypass](docs/ROLLBACK.md)
- [API v1](docs/API.md)
- [Operations](docs/OPERATIONS.md)

Build with Go 1.27.2, Python/jsonschema and ShellCheck:

```sh
export SING_BOX_BIN=$(sh scripts/fetch-sing-box-checker.sh amd64)
make verify
python3 scripts/check_contracts.py
sh tests/openwrt/check_overlay.sh
sh scripts/package.sh
sh tests/package/check_archive.sh
```

The versioned ARM64 archive, per-file manifest and SHA-256 record appear in dist/. The OS and sing-box remain prerequisites; production credentials are never packaged.

## Repository policy

- Default branch: `main`.
- No private keys, UUIDs, server credentials, subscription URLs, access tokens, `.env` files, or production configuration containing secrets may be committed.
- Example configuration must use unmistakable placeholders.
- Deployment changes must be reproducible and reversible.
- Shared contribution/security defaults are inherited from `NikaSir/.github` unless overridden here.

## Target layout

```text
config/
docs/
scripts/
cmd/
internal/
openwrt/
tests/
.github/workflows/
```

Apply the prepared topology only within the approved maintenance window after reviewing the generated network plan and rollback command.
