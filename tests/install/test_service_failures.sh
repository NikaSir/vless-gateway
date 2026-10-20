#!/bin/sh
set -eu
repo=$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)
# shellcheck source=scripts/lib/services.sh
. "$repo/scripts/lib/services.sh"
test_root=$(mktemp -d)
trap 'rm -r "$test_root"' EXIT HUP INT TERM
mkdir -p "$test_root/bin" "$test_root/services"
cat > "$test_root/operation" <<'EOF'
#!/bin/sh
set -eu
name=${0##*/}
printf '%s %s\n' "$name" "$*" >> "$NIKAS_OPERATION_LOG"
test "$(wc -l < "$NIKAS_OPERATION_LOG")" -ne "$NIKAS_FAIL_OPERATION"
EOF
chmod 755 "$test_root/operation"
cp "$test_root/operation" "$test_root/bin/uci"
for name in network firewall dnsmasq nikas-gatewayd nikas-sing-box; do cp "$test_root/operation" "$test_root/services/$name"; done
export PATH="$test_root/bin:$PATH"
export NIKAS_OPERATION_LOG="$test_root/operations.log"
for step in 1 2 3 4 5 6 7 8 9 10 11 12 13; do
 : > "$NIKAS_OPERATION_LOG"
 export NIKAS_FAIL_OPERATION=$step
 if apply_services "$test_root/services" policy; then printf 'Operation %s did not propagate failure\n' "$step"; exit 1; fi
 test "$(wc -l < "$NIKAS_OPERATION_LOG")" -eq "$step"
done
export NIKAS_FAIL_OPERATION=0
: > "$NIKAS_OPERATION_LOG"
apply_services "$test_root/services" policy
test "$(wc -l < "$NIKAS_OPERATION_LOG")" -eq 13
printf '%s\n' 'Every UCI/service failure propagates to installer rollback'
