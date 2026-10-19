#!/bin/sh
set -eu
repo=$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)
cd "$repo"
test_root=$(mktemp -d)
trap 'rm -r "$test_root"' EXIT HUP INT TERM
mkdir -p "$test_root/release/bin" "$test_root/device" "$test_root/release/rules/generations/initial"
go build -o "$test_root/release/bin/gatewayctl" ./cmd/gatewayctl
go build -o "$test_root/release/bin/gatewayd" ./cmd/gatewayd
cp -Rp scripts openwrt config VERSION "$test_root/release/"
python3 tests/install/fixtures/prepare.py "$repo" "$test_root/input"
ctl="$test_root/release/bin/gatewayctl"
"$ctl" --config-root "$test_root/input" init-auth
# Missing DIRECT acceptance must fail before changing the destination.
if NIKAS_ROOT="$test_root/device" sh scripts/install.sh --config-dir "$test_root/input" --secrets-file "$test_root/input/secrets.json" --inventory "$test_root/input/inventory.json" --stage policy 2>/dev/null; then exit 1; fi
test ! -e "$test_root/device/etc"
for name in geosite-ru geoip-ru force-direct force-vless; do
 "${SING_BOX_BIN:-sing-box}" rule-set compile config/examples/force-direct.example.json -o "$test_root/release/rules/generations/initial/$name.srs"
done
printf '%s\n' '{"generation":"initial","files":[]}' > "$test_root/release/rules/generations/initial/manifest.json"
ln -s generations/initial "$test_root/release/rules/current"
sing_box_dir=$(dirname "${SING_BOX_BIN:-$(command -v sing-box)}")
PATH="$sing_box_dir:$PATH" NIKAS_ROOT="$test_root/device" sh "$test_root/release/scripts/install.sh" --config-dir "$test_root/input" --secrets-file "$test_root/input/secrets.json" --inventory "$test_root/input/inventory.json" > "$test_root/install.log"
secret="$test_root/device/etc/nikas-vless/secrets.json"
before=$(sha256sum "$secret" | cut -d ' ' -f 1)
PATH="$sing_box_dir:$PATH" NIKAS_ROOT="$test_root/device" sh "$test_root/release/scripts/install.sh" --config-dir "$test_root/input" --secrets-file /missing --inventory "$test_root/input/inventory.json" > "$test_root/reinstall.log"
after=$(sha256sum "$secret" | cut -d ' ' -f 1)
test "$before" = "$after"
test "$(stat -c %a "$secret")" = 600
test -f "$test_root/device/etc/nikas-vless/network-plan/etc/config/network"
test ! -f "$test_root/device/etc/config/network"
printf '%s\n' 'Install is idempotent; production secrets preserved; topology awaits explicit application'
