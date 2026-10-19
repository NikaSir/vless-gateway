#!/bin/sh
# Required CI integration: real kernel state and dnsmasq, isolated from host networking.
set -eu
if [ "${1:-}" != --inside ]; then
 exec unshare --net sh "$0" --inside
fi
repo=$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)
test_root=$(mktemp -d)
wrapper=
fallback=
dnsmasq_pid=
cleanup() {
 trap - EXIT HUP INT TERM
 for pid in "$wrapper" "$fallback" "$dnsmasq_pid"; do [ -z "$pid" ] || kill "$pid" 2>/dev/null || :; done
 rm -r "$test_root"
}
trap cleanup EXIT HUP INT TERM
ip link set lo up
mkdir -p "$test_root/bin" "$test_root/usr/libexec"
cp "$repo/openwrt/usr/libexec/nikas-interception-cleanup" "$test_root/usr/libexec/"
cat > "$test_root/bin/sing-box" <<'EOF'
#!/bin/sh
exec python3 "$NIKAS_DNS_FIXTURE" 127.0.0.2 192.0.2.10 "$NIKAS_CHILD_PID"
EOF
chmod 755 "$test_root/bin/sing-box" "$test_root/usr/libexec/nikas-interception-cleanup"
export NIKAS_DNS_FIXTURE="$repo/tests/openwrt/fixtures/dns-resolver.py"
export NIKAS_CHILD_PID="$test_root/child.pid"
python3 "$NIKAS_DNS_FIXTURE" 127.0.0.3 192.0.2.20 &
fallback=$!
dnsmasq --keep-in-foreground --user=root --no-resolv --no-hosts --cache-size=0 --strict-order --bind-interfaces --listen-address=127.0.0.1 --port=1053 --server=127.0.0.2 --server=127.0.0.3 --pid-file="$test_root/dnsmasq.pid" --log-facility="$test_root/dnsmasq.log" &
dnsmasq_pid=$!
NIKAS_ROOT="$test_root" PATH="$test_root/bin:$PATH" sh "$repo/openwrt/usr/libexec/nikas-sing-box-run" &
wrapper=$!
python3 "$repo/tests/openwrt/fixtures/query-dns.py" 192.0.2.10
nft add table inet sing-box
nft add table inet unrelated
ip -4 rule add priority 9020 table 20220
ip -4 rule add priority 32760 table 20220
ip -4 rule add priority 12345 table 12345
ip -4 route add blackhole 198.51.100.0/24 table 20220
ip -4 route add blackhole 203.0.113.0/24 table 12345
kill -KILL "$(cat "$NIKAS_CHILD_PID")"
if wait "$wrapper"; then printf '%s\n' 'Killed child unexpectedly succeeded'; exit 1; fi
wrapper=
python3 "$repo/tests/openwrt/fixtures/query-dns.py" 192.0.2.20
kill -0 "$dnsmasq_pid"
if nft list table inet sing-box >/dev/null 2>&1; then exit 1; fi
nft list table inet unrelated >/dev/null
ip -j -4 rule show > "$test_root/rules.json"
ip -j -4 route show table 20220 > "$test_root/reserved.json"
ip -j -4 route show table 12345 > "$test_root/unrelated.json"
python3 - "$test_root" <<'PY'
import json
from pathlib import Path
import sys
root = Path(sys.argv[1])
priorities = {r["priority"] for r in json.loads((root / "rules.json").read_text())}
assert 12345 in priorities and not priorities.intersection(range(9020, 9040)) and 32760 not in priorities
assert not json.loads((root / "reserved.json").read_text())
assert any(r["dst"] == "203.0.113.0/24" for r in json.loads((root / "unrelated.json").read_text()))
PY
printf '%s\n' 'Real DNS fallback and reserved-only kernel cleanup passed'
