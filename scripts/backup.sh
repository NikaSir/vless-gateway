#!/bin/sh
set -eu
exec "${GATEWAYCTL:-gatewayctl}" backup --system-root "${NIKAS_ROOT:-/}" "$@"
