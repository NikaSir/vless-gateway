#!/bin/sh
# Installs a prepared release archive. Live topology is applied only with --apply-network.
set -eu
umask 077
base=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
# shellcheck source=scripts/lib/common.sh
. "$base/scripts/lib/common.sh"
# shellcheck source=scripts/lib/services.sh
. "$base/scripts/lib/services.sh"
require_root
system_root=${NIKAS_ROOT:-/}
case "$system_root" in /*) ;; *) printf '%s\n' 'Absolute NIKAS_ROOT required' >&2; exit 1;; esac
runtime="$system_root/etc/nikas-vless"
config_dir=
secret_file=
inventory=
stage=direct
apply_network=0
while [ "$#" -gt 0 ]; do
 case "$1" in
  --config-dir) config_dir=$2; shift 2;;
  --secrets-file) secret_file=$2; shift 2;;
  --inventory) inventory=$2; shift 2;;
  --stage) stage=$2; shift 2;;
  --apply-network) apply_network=1; shift;;
  *) printf '%s\n' 'Unknown installer option' >&2; exit 1;;
 esac
done
case "$stage" in direct|policy) ;; *) printf '%s\n' 'Invalid stage' >&2; exit 1;; esac
[ -n "$config_dir" ] || { printf '%s\n' '--config-dir required' >&2; exit 1; }
[ ! -L "$runtime" ] || { printf '%s\n' 'Runtime directory must not be a symlink' >&2; exit 1; }
ctl=${GATEWAYCTL:-$base/bin/gatewayctl}
[ -x "$ctl" ] || { printf '%s\n' 'Release gatewayctl unavailable' >&2; exit 1; }
work=$(mktemp -d)
backup_path=
rollback_needed=0
transaction="$system_root/etc/nikas-vless-install.transaction"
cleanup_install() {
 install_status=$?
 trap - EXIT HUP INT TERM
 if [ "$rollback_needed" -eq 1 ]; then
  printf '%s\n' 'Installation interrupted or failed; restoring local backup' >&2
  if "$ctl" restore --system-root "$system_root" --backup "$backup_path" --yes; then
   rm -f "$transaction"
   sync
  else
   printf '%s\n' 'Automatic restore failed; transaction retained for recovery' >&2
  fi
  [ "$install_status" -ne 0 ] || install_status=1
 fi
 rm -r "$work"
 exit "$install_status"
}
trap cleanup_install EXIT
trap 'exit 130' HUP INT TERM
# A durable marker survives SIGKILL/power loss. Recover before new preflight/writes.
if [ -e "$transaction" ] || [ -L "$transaction" ]; then
 [ -f "$transaction" ] && [ ! -L "$transaction" ] || { printf '%s\n' 'Unsafe installation journal' >&2; exit 1; }
 IFS= read -r pending_backup < "$transaction"
 case "$pending_backup" in "${system_root%/}"/etc/nikas-vless/backups/*.tar.gz) ;; *) printf '%s\n' 'Invalid installation journal' >&2; exit 1;; esac
 "$ctl" restore --system-root "$system_root" --backup "$pending_backup" --yes
 rm -f "$transaction"
 sync
fi
cp "$config_dir/settings.json" "$work/settings.json"
cp "$config_dir/hardware.json" "$work/hardware.json"
if [ -f "$runtime/secrets.json" ]; then cp "$runtime/secrets.json" "$work/secrets.json"; else
 [ -n "$secret_file" ] || { printf '%s\n' '--secrets-file required for first install' >&2; exit 1; }
 cp "$secret_file" "$work/secrets.json"
fi
if [ -f "$runtime/auth.json" ]; then cp "$runtime/auth.json" "$work/auth.json"; else "$ctl" --config-root "$work" init-auth; fi
"$ctl" --config-root "$work" validate-input
if [ "$apply_network" -eq 1 ]; then
 [ -s "$system_root/etc/dropbear/authorized_keys" ] || { printf '%s\n' 'Verified SSH key required before applying topology' >&2; exit 1; }
fi
set --
if [ -n "$inventory" ]; then set -- "$@" --inventory "$inventory"; fi
if [ -f "$runtime/installed-version" ]; then set -- "$@" --owned; fi
"$ctl" --config-root "$work" preflight "$@" --output "$work/network-plan"
if [ "$stage" = policy ]; then
 [ -f "$runtime/state/direct-accepted.json" ] || { printf '%s\n' 'DIRECT acceptance evidence missing' >&2; exit 1; }
 [ "$apply_network" -eq 1 ] || { printf '%s\n' '--apply-network required for policy stage' >&2; exit 1; }
fi
backup_path=$("$ctl" backup --system-root "$system_root")
printf 'Rollback: gatewayctl restore --system-root %s --backup %s --yes\n' "$system_root" "$backup_path"
rollback_install() {
 exit 1
}
rollback_needed=1
printf '%s\n' "$backup_path" > "$work/install.transaction"
atomic_copy "$work/install.transaction" "$transaction"
sync
# No live write occurs before preflight and the backup above.
mkdir -p "$runtime/state" "$runtime/overrides"
for name in settings hardware auth secrets; do
 if [ "$name" = secrets ] && [ -f "$runtime/secrets.json" ]; then continue; fi
 atomic_copy "$work/$name.json" "$runtime/$name.json" || rollback_install
 chmod 600 "$runtime/$name.json"
done
if [ ! -L "$runtime/rules/current" ]; then
 mkdir -p "$runtime/rules"
 cp -Rp "$base/rules/." "$runtime/rules/" || rollback_install
fi
for name in force-direct force-vless; do
 [ -f "$runtime/overrides/$name.json" ] || atomic_copy "$base/config/examples/$name.example.json" "$runtime/overrides/$name.json"
done
for name in gatewayd gatewayctl; do atomic_copy "$base/bin/$name" "$system_root/usr/bin/$name" || rollback_install; done
(cd "$base/openwrt" && find . -type f) | while IFS= read -r file; do atomic_copy "$base/openwrt/$file" "$system_root/$file" || exit 1; done || rollback_install
mkdir -p "$system_root/usr/libexec/nikas-vless"
for name in backup restore doctor preflight; do atomic_copy "$base/scripts/$name.sh" "$system_root/usr/libexec/nikas-vless/$name.sh" || rollback_install; done
"$ctl" --config-root "$runtime" render --output "$runtime/config.json" || rollback_install
"$ctl" --config-root "$runtime" validate || rollback_install
mkdir -p "$runtime/network-plan"
cp -Rp "$work/network-plan/." "$runtime/network-plan/" || rollback_install
if [ "$apply_network" -eq 1 ]; then
 (cd "$runtime/network-plan" && find . -type f) | while IFS= read -r file; do atomic_copy "$runtime/network-plan/$file" "$system_root/$file" || exit 1; done || rollback_install
 if [ "$system_root" = / ]; then
  apply_services /etc/init.d "$stage" || rollback_install
 fi
fi
atomic_copy "$base/VERSION" "$runtime/installed-version"
rm -f "$transaction"
sync
rollback_needed=0
printf '%s\n' 'Installed. DIRECT-first acceptance is required before enabling policy.'
