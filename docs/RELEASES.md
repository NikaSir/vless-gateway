# Release policy

## Source of truth

- `main` is the canonical source branch.
- Every release must be traceable to an immutable Git commit/tag.
- Deployment artifacts must be produced from committed source, never from an uncommitted local working tree.

## Version lineage

Use MAJOR.MINOR.PATCH-betaNNN, starting at 1.0.0-beta001. Both binaries and every API response display the full version. Published history is immutable.

The first archive is nikas-vless-gateway-1.0.0-beta001-aarch64.tar.gz with a SHA-256 companion and per-file manifest. Go 1.27.2, sing-box 1.12.22 and initial RU source commits/checksums are pinned. Rebuild twice from the same clean commit and compare archives byte-for-byte.

No release tag before bench acceptance. Green repository checks do not replace the device evidence in docs/ACCEPTANCE.md. Owner approval is required before tagging or applying live policy.

## Release gate

Before a release:

1. Repository checks are green.
2. Deployment has been tested on the supported target hardware/OS.
3. Rollback or recovery steps are documented and tested for material networking changes.
4. `CHANGELOG.md` is updated.
5. No private keys, UUIDs, subscription URLs, access tokens, production `.env` files, or other credentials are present in tracked files or artifacts.
6. The release tag points to the exact reviewed commit.

Published tags are treated as immutable.
