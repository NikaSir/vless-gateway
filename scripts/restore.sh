#!/bin/sh
set -eu
exec "${GATEWAYCTL:-gatewayctl}" restore --system-root "${NIKAS_ROOT:-/}" "$@"
