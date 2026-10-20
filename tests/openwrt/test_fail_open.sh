#!/bin/sh
set -eu
repo=$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)
test_root=$(mktemp -d)
trap 'rm -r "$test_root"' EXIT HUP INT TERM
mkdir -p "$test_root/bin" "$test_root/usr/libexec"
export NIKAS_TEST_LOG="$test_root/log"
# Command shims are generated only in a private test directory.
cp "$repo/tests/openwrt/fixtures/command-shim" "$test_root/bin/nft"
cp "$repo/tests/openwrt/fixtures/command-shim" "$test_root/bin/ip"
chmod 755 "$test_root/bin/"*
PATH="$test_root/bin:$PATH" sh "$repo/openwrt/usr/libexec/nikas-interception-cleanup"
test "$(wc -l < "$test_root/log")" -eq 23
if grep -Eq 'flush ruleset|flush table inet fw4|masquerade|snat' "$test_root/log"; then exit 1; fi
grep -q 'nft delete table inet sing-box' "$test_root/log"
grep -q 'ip -4 rule del priority 9020' "$test_root/log"
grep -q 'ip -4 route flush table 20220' "$test_root/log"
cp "$repo/openwrt/usr/libexec/nikas-interception-cleanup" "$test_root/usr/libexec/"
cp "$repo/tests/openwrt/fixtures/sing-box-shim" "$test_root/bin/sing-box"
cp "$repo/tests/openwrt/fixtures/command-shim" "$test_root/bin/logger"
chmod 755 "$test_root/bin/"* "$test_root/usr/libexec/"*
export NIKAS_TEST_PID="$test_root/pid"
NIKAS_ROOT="$test_root" PATH="$test_root/bin:$PATH" sh "$repo/openwrt/usr/libexec/nikas-sing-box-run" &
wrapper=$!
attempt=0
while [ ! -s "$test_root/pid" ] && [ "$attempt" -lt 100 ]; do sleep 0.02; attempt=$((attempt + 1)); done
test -s "$test_root/pid"
kill -KILL "$(cat "$test_root/pid")"
if wait "$wrapper"; then printf '%s\n' 'Unexpected successful wrapper exit'; exit 1; fi
test "$(grep -c 'nft delete table inet sing-box' "$test_root/log")" -eq 3
printf '%s\n' 'Fail-open cleanup changed only reserved resources'
