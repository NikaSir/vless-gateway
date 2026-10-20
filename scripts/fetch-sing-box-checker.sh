#!/bin/sh
set -eu
repo=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
arch=${1:-amd64}
case "$arch" in amd64|arm64) ;; *) exit 1;; esac
name="sing-box-1.12.22-linux-$arch.tar.gz"
mkdir -p "$repo/.tools"
archive="$repo/.tools/$name"
if [ ! -f "$archive" ]; then curl -fsSL --connect-timeout 30 --max-time 180 "https://github.com/SagerNet/sing-box/releases/download/v1.12.22/$name" -o "$archive.pending"; mv "$archive.pending" "$archive"; fi
expected=$(awk -v name="$name" '$2 == name {print $1}' "$repo/tools/checksums/sing-box-1.12.22.sha256")
actual=$(sha256sum "$archive" | cut -d ' ' -f 1)
[ -n "$expected" ] && [ "$actual" = "$expected" ] || { printf '%s\n' 'sing-box checksum mismatch' >&2; exit 1; }
tar --no-same-owner -xzf "$archive" -C "$repo/.tools"
binary="$repo/.tools/sing-box-1.12.22-linux-$arch/sing-box"
if [ "$arch" = amd64 ]; then
 "$binary" version | grep -q '^sing-box version 1.12.22$'
 "$binary" version | grep -q 'with_clash_api'
fi
printf '%s\n' "$binary"
