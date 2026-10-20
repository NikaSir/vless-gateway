#!/bin/sh
set -eu
repo=$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)
python3 "$repo/tests/package/check_archive.py" "${1:-$repo/dist/nikas-vless-gateway-1.0.0-beta001-aarch64.tar.gz}"
