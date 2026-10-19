#!/bin/sh
set -eu
repo=$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)
test_root=$(mktemp -d)
trap 'rm -r "$test_root"' EXIT HUP INT TERM
mkdir -p "$test_root/bin"
cat > "$test_root/bin/gatewayctl" <<'EOF'
#!/bin/sh
printf '%s\n' recovery >> "$NIKAS_RECOVERY_LOG"
EOF
cat > "$test_root/bin/uci" <<'EOF'
#!/bin/sh
printf '%s\n' 1
EOF
cat > "$test_root/bin/sing-box" <<'EOF'
#!/bin/sh
exit 0
EOF
chmod 755 "$test_root/bin/"*
export PATH="$test_root/bin:$PATH" NIKAS_RECOVERY_LOG="$test_root/recovery.log"
procd_open_instance(){ :; }
procd_set_param(){ :; }
procd_close_instance(){ :; }
# shellcheck disable=SC1091
. "$repo/openwrt/etc/init.d/nikas-sing-box"
start_service
test "$(wc -l < "$NIKAS_RECOVERY_LOG")" -eq 1
NIKAS_CONFIG_ACTIVATION=1 start_service
test "$(wc -l < "$NIKAS_RECOVERY_LOG")" -eq 1
start_service
test "$(wc -l < "$NIKAS_RECOVERY_LOG")" -eq 2
printf '%s\n' 'Ordinary startup recovers; in-transaction reload keeps the candidate for health acknowledgement'
