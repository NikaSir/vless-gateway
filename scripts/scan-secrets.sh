#!/bin/sh
set -eu
repo=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
if [ "${1:-}" = --self-test ]; then exec python3 "$repo/tests/package/test_secret_scan.py"; fi
exec python3 "$repo/scripts/scan_secrets.py" "$@"
