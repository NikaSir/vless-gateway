#!/bin/sh
set -eu
umask 077
require_root() { [ "$(id -u)" -eq 0 ] || { printf '%s\n' 'Root required' >&2; exit 1; }; }
atomic_copy() {
 source_file=$1
 destination=$2
 mkdir -p "$(dirname "$destination")"
 cp -p "$source_file" "$destination.nikas-pending"
 mv -f "$destination.nikas-pending" "$destination"
}
