#!/bin/sh
set -eu
repo=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$repo"
version=$(tr -d '\n' < VERSION)
epoch=${SOURCE_DATE_EPOCH:-$(git log -1 --format=%ct)}
stage=$(mktemp -d)
trap 'rm -r "$stage"' EXIT HUP INT TERM
make build-arm64
mkdir -p "$stage/bin" "$stage/rules/generations/initial" "$repo/dist"
cp dist/gatewayd dist/gatewayctl "$stage/bin/"
cp -Rp openwrt config VERSION LICENSE "$stage/"
mkdir -p "$stage/scripts/lib"
cp scripts/install.sh scripts/preflight.sh scripts/backup.sh scripts/restore.sh scripts/doctor.sh "$stage/scripts/"
cp scripts/lib/common.sh scripts/lib/services.sh "$stage/scripts/lib/"
if [ -d docs ]; then cp -Rp docs "$stage/"; fi
# Execution plans are developer artifacts, not device installation inputs.
if [ -d "$stage/docs/superpowers" ]; then rm -r "$stage/docs/superpowers"; fi
checker=${SING_BOX_BIN:-$(sh scripts/fetch-sing-box-checker.sh amd64)}
python3 scripts/fetch_rules.py "$stage/rules/generations/initial"
for name in force-direct force-vless; do "$checker" rule-set compile "config/examples/$name.example.json" -o "$stage/rules/generations/initial/$name.srs"; done
ln -s generations/initial "$stage/rules/current"
python3 scripts/build_manifest.py "$stage" "$repo" "$epoch"
python3 scripts/scan_secrets.py "$stage"
archive="$repo/dist/nikas-vless-gateway-$version-aarch64.tar.gz"
tar --sort=name --mtime="@$epoch" --owner=0 --group=0 --numeric-owner -C "$stage" -cf "$stage/archive.tar" bin config docs scripts openwrt rules VERSION LICENSE manifest.json
gzip -n -c "$stage/archive.tar" > "$archive.pending"
mv "$archive.pending" "$archive"
(cd "$repo/dist" && sha256sum "${archive##*/}" > "${archive##*/}.sha256")
printf '%s\n' "$archive"
