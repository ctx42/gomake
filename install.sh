#!/usr/bin/env sh
# gomake installer — requires Go 1.26+
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/ctx42/gomake/master/install.sh | sh
#
# With a custom targets.yaml (local path or HTTPS URL):
#   curl -fsSL ... | sh -s -- --targets=path/to/targets.yaml
#   curl -fsSL ... | sh -s -- --targets=http://example.com/targets.yaml
#
set -eu

if ! command -v go >/dev/null 2>&1; then
    printf 'error: go is required\ninstall it from https://go.dev/dl/\n' >&2
    exit 1
fi

exec go run github.com/ctx42/gomake/cmd/install@latest "$@"
