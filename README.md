# VLESS Gateway

Standalone **network gateway** project for deploying and operating the VLESS-based gateway appliance.

## Scope boundary

This repository contains the gateway itself: host provisioning, network/service configuration, deployment logic, diagnostics, backup/restore procedures, and operational documentation.

The Home Assistant integration is a separate product and lives in **`NikaSir/ha-vless-gateway`**. Home Assistant-specific code must not be mixed into this repository.

## Target platform

Initial project target: Raspberry Pi based deployment. Hardware-specific assumptions will be documented explicitly rather than embedded silently in scripts.

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
systemd/
tests/
.github/workflows/
```

Operational implementation will be added as the gateway build is validated on real hardware.
