#!/bin/sh
set -eu
exec "${GATEWAYCTL:-gatewayctl}" --config-root "${NIKAS_CONFIG_ROOT:-/etc/nikas-vless}" doctor "$@"
