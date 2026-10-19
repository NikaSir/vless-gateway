#!/bin/sh
set -eu
repo=$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)
cd "$repo"
go test ./internal/backup -count=1
