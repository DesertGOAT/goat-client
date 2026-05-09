#!/usr/bin/env bash
# build-linux-pkg.sh — driver for the deb + rpm nfpm builds.
#
# nfpm v2.x does not expand ${VAR} inside contents.src globs, so we
# envsubst the config first into a tmp file, then hand that to nfpm.
# (Other fields like `arch:` and `version:` *do* honor env vars, but
# the contents.src pass is regex-only.)
#
# Usage:
#   VERSION=0.0.1 GOARCH=amd64 packaging/build-linux-pkg.sh deb
#   VERSION=0.0.1 GOARCH=arm64 packaging/build-linux-pkg.sh rpm
#
# Inputs (Track E supplies before this runs):
#   dist/linux_${GOARCH}/goat-clientd
#   dist/linux_${GOARCH}/goat-client
#
# Output:
#   dist/<package-file>

set -euo pipefail

: "${VERSION:?VERSION env var required}"
: "${GOARCH:?GOARCH env var required (amd64|arm64)}"

format="${1:-}"
case "$format" in
    deb|rpm) ;;
    *)
        echo "usage: $0 (deb|rpm)" >&2
        exit 64
        ;;
esac

mkdir -p dist
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

VERSION="$VERSION" GOARCH="$GOARCH" \
    envsubst < "packaging/${format}/nfpm.yaml" > "$tmp"

nfpm pkg --config "$tmp" --packager "$format" --target dist/
