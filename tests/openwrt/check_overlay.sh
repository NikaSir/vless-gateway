#!/bin/sh
set -eu
repo=$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)
for file in "$repo"/openwrt/etc/init.d/* "$repo"/openwrt/usr/libexec/* "$repo"/openwrt/etc/hotplug.d/iface/*; do
 sh -n "$file"
done
sh "$repo/tests/openwrt/test_fail_open.sh"
sh "$repo/tests/openwrt/test_config_recovery.sh"
python3 "$repo/tests/openwrt/test_dns_fixture.py"
