#!/bin/sh
set -eu
repo=$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)
test_root=$(mktemp -d)
trap 'rm -r "$test_root"' EXIT HUP INT TERM
mkdir -p "$test_root/bin"
cat > "$test_root/bin/gatewayctl" <<'EOF'
#!/bin/sh
printf '%s\n' "$3" >> "$NIKAS_RECOVERY_LOG"
[ "${NIKAS_FAIL_RECOVERY:-}" != "$3" ]
EOF
cat > "$test_root/bin/uci" <<'EOF'
#!/bin/sh
printf '%s\n' 1
EOF
cat > "$test_root/bin/sing-box" <<'EOF'
#!/bin/sh
printf '%s\n' check >> "$NIKAS_RECOVERY_LOG"
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
printf '%s\n' recover-config recover-rules check > "$test_root/expected"
cmp "$test_root/expected" "$NIKAS_RECOVERY_LOG"
NIKAS_CONFIG_ACTIVATION=1 start_service
printf '%s\n' check >> "$test_root/expected"
cmp "$test_root/expected" "$NIKAS_RECOVERY_LOG"
start_service
printf '%s\n' recover-config recover-rules check >> "$test_root/expected"
cmp "$test_root/expected" "$NIKAS_RECOVERY_LOG"
if NIKAS_FAIL_RECOVERY=recover-rules start_service; then
 echo 'Failed recovery accepted' >&2
 exit 1
fi
printf '%s\n' recover-config recover-rules >> "$test_root/expected"
cmp "$test_root/expected" "$NIKAS_RECOVERY_LOG"
printf '%s\n' 'Ordinary startup recovers config and rules before checking; transaction restart preserves candidate; recovery failure blocks startup'
