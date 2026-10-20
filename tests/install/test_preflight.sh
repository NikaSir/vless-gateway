#!/bin/sh
set -eu
repo=$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)
cd "$repo"
go test ./internal/provision -count=1
