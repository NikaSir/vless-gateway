#!/bin/sh
# Checked service operations; caller owns transaction rollback on any nonzero result.
apply_services() {
 service_dir=$1
 policy_stage=$2
 uci set 'dropbear.@dropbear[0].PasswordAuth=off' || return 1
 uci set 'dropbear.@dropbear[0].RootPasswordAuth=off' || return 1
 uci commit dropbear || return 1
 "$service_dir/nikas-sing-box" stop || return 1
 "$service_dir/network" restart || return 1
 "$service_dir/firewall" restart || return 1
 "$service_dir/dnsmasq" restart || return 1
 "$service_dir/nikas-gatewayd" enable || return 1
 "$service_dir/nikas-gatewayd" restart || return 1
 "$service_dir/nikas-sing-box" enable || return 1
 if [ "$policy_stage" = policy ]; then
  uci set nikas-vless.main.policy_enabled=1 || return 1
  uci commit nikas-vless || return 1
  "$service_dir/nikas-sing-box" start || return 1
 fi
}
