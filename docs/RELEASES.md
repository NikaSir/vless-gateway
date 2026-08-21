# Release policy

## Source of truth

- `main` is the canonical source branch.
- Every release must be traceable to an immutable Git commit/tag.
- Deployment artifacts must be produced from committed source, never from an uncommitted local working tree.

## Version lineage

Repository bootstrap is not a functional gateway release. The public version scheme will be fixed when the first validated gateway build is ready; published history must not later be renumbered merely for cosmetic consistency.

## Release gate

Before a release:

1. Repository checks are green.
2. Deployment has been tested on the supported target hardware/OS.
3. Rollback or recovery steps are documented and tested for material networking changes.
4. `CHANGELOG.md` is updated.
5. No private keys, UUIDs, subscription URLs, access tokens, production `.env` files, or other credentials are present in tracked files or artifacts.
6. The release tag points to the exact reviewed commit.

Published tags are treated as immutable.
