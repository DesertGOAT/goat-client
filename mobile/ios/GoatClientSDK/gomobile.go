//go:build tools

// This file is never compiled during a normal build — the `tools` build tag
// is intentionally never set. Its sole purpose is to keep
// golang.org/x/mobile/bind on the go.mod / go.sum graph so that
// `gomobile bind` (driven by mobile/ios/scripts/build-xcframework.sh) finds
// the package when generating the Objective-C/Swift bridge.
//
// Avoiding the import in normally-compiled files sidesteps a known issue
// where `golang.org/x/tools` (transitive dep of x/mobile/bind) fails to
// type-check under GOOS=ios GOARCH=arm64 on the host toolchain. The real
// gomobile bind invocation uses its own cross-compile pipeline which is
// unaffected.
package GoatClientSDK

import _ "golang.org/x/mobile/bind"
