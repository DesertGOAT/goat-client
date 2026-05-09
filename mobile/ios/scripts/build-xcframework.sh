#!/usr/bin/env bash
# build-xcframework.sh — build GoatClientSDK.xcframework via gomobile bind.
#
# Output: mobile/ios/GoatClientSDK.xcframework, with both ios-arm64 (device)
# and ios-arm64_x86_64-simulator (Simulator) slices, ready for the Xcode
# project at mobile/ios/Shell/ to link against.
#
# Prerequisites (one-time, on the build host):
#   - Xcode + command-line tools:  xcode-select --install
#   - Go 1.23+
#   - gomobile + gobind binaries:
#         go install golang.org/x/mobile/cmd/gomobile@latest
#         go install golang.org/x/mobile/cmd/gobind@latest
#         "$(go env GOPATH)/bin/gomobile" init
#
# Usage (from repo root):
#   ./mobile/ios/scripts/build-xcframework.sh
#
# Useful env-var overrides:
#   BUNDLE_ID  — override the Objective-C namespace prefix (default
#                io.dlf-dds.goat-client.framework)
#   OUTPUT     — override the .xcframework output path
#   IOS_TARGET — override the gomobile -target flag (default ios,iossimulator)
#                Set to "iossimulator" alone for a Simulator-only build (no
#                Apple Developer team needed).

set -euo pipefail

# Resolve repo root regardless of cwd.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

BUNDLE_ID="${BUNDLE_ID:-io.dlf-dds.goat-client.framework}"
OUTPUT="${OUTPUT:-$REPO_ROOT/mobile/ios/GoatClientSDK.xcframework}"
IOS_TARGET="${IOS_TARGET:-ios,iossimulator}"

GO_PKG="github.com/dlf-dds/goat-client/mobile/ios/GoatClientSDK"

# Sanity: gomobile must be on PATH.
if ! command -v gomobile >/dev/null 2>&1; then
    GOPATH_BIN="$(go env GOPATH)/bin"
    if [[ -x "$GOPATH_BIN/gomobile" ]]; then
        export PATH="$GOPATH_BIN:$PATH"
    else
        echo "error: gomobile not found on PATH and not at $GOPATH_BIN/gomobile" >&2
        echo "       install with: go install golang.org/x/mobile/cmd/gomobile@latest" >&2
        echo "       then run:    gomobile init" >&2
        exit 1
    fi
fi

cd "$REPO_ROOT"

# Wipe any previous output so gomobile bind creates a fresh xcframework.
rm -rf "$OUTPUT"

echo "==> gomobile bind"
echo "    target:    $IOS_TARGET"
echo "    bundleid:  $BUNDLE_ID"
echo "    package:   $GO_PKG"
echo "    output:    $OUTPUT"

# -trimpath + -ldflags=-s/-w: stripped, reproducible-ish binaries.
gomobile bind \
    -target="$IOS_TARGET" \
    -bundleid="$BUNDLE_ID" \
    -o "$OUTPUT" \
    -trimpath \
    -ldflags='-s -w' \
    "$GO_PKG"

echo "==> built: $OUTPUT"

# Sanity: list what slices got produced.
if [[ -d "$OUTPUT" ]]; then
    echo "==> xcframework slices:"
    find "$OUTPUT" -maxdepth 2 -mindepth 2 -type d | sort
fi
