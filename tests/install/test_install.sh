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

cat > "$test_root/fault-ctl" <<'EOF'
#!/bin/sh
case " $* " in
 *' render '*)
  case "$NIKAS_TEST_FAULT" in
   fail) exit 42;;
   term) kill -TERM "$PPID"; exit 0;;
   kill) kill -KILL "$PPID"; exit 0;;
  esac;;
esac
exec "$NIKAS_REAL_CTL" "$@"
EOF
chmod 755 "$test_root/fault-ctl"
export NIKAS_REAL_CTL="$ctl"
mkdir -p "$test_root/fault-bin"
cat > "$test_root/fault-bin/cp" <<'EOF'
#!/bin/sh
for argument in "$@"; do
 case "$NIKAS_TEST_FAULT:$argument" in
  topology:*/etc/config/firewall.nikas-pending|final:*/installed-version.nikas-pending) exit 43;;
 esac
done
exec /bin/cp "$@"
EOF
chmod 755 "$test_root/fault-bin/cp"
for fault in fail term kill topology final; do
 device="$test_root/$fault-device"
 mkdir -p "$device/etc/config" "$device/etc/dropbear"
 printf '%s\n' original-network > "$device/etc/config/network"
 printf '%s\n' fixture-ssh-key > "$device/etc/dropbear/authorized_keys"
 set --
 if [ "$fault" = topology ]; then set -- --apply-network; fi
 if TMPDIR="$test_root" PATH="$test_root/fault-bin:$sing_box_dir:$PATH" NIKAS_ROOT="$device" GATEWAYCTL="$test_root/fault-ctl" NIKAS_TEST_FAULT="$fault" sh "$test_root/release/scripts/install.sh" --config-dir "$test_root/input" --secrets-file "$test_root/input/secrets.json" --inventory "$test_root/input/inventory.json" "$@" > "$test_root/$fault.log" 2>&1; then
  printf 'Failure injection unexpectedly succeeded: %s\n' "$fault"; exit 1
 fi
 if [ "$fault" = kill ]; then
  test -f "$device/etc/nikas-vless-install.transaction"
  # Recovery must happen before a new, deliberately rejected preflight.
  if PATH="$sing_box_dir:$PATH" NIKAS_ROOT="$device" sh "$test_root/release/scripts/install.sh" --config-dir "$test_root/input" --secrets-file "$test_root/input/secrets.json" --inventory /missing > "$test_root/recovery.log" 2>&1; then exit 1; fi
 fi
 test "$(cat "$device/etc/config/network")" = original-network
 test ! -f "$device/etc/init.d/nikas-sing-box"
 test ! -f "$device/etc/nikas-vless/settings.json"
 test ! -f "$device/usr/bin/gatewayd"
 test ! -e "$device/etc/nikas-vless-install.transaction"
 test -n "$(find "$device/etc/nikas-vless/backups" -name '*.tar.gz')"
done
printf '%s\n' 'Publication failure, final write failure, TERM and interrupted-install recovery restore the original state'
