//go:build android

package goatclient

// Pulls golang.org/x/mobile/bind into the build graph for `gomobile bind`.
// gomobile inspects this package's import set when generating Java/Kotlin
// stubs; without this anchor the bind step would not see the toolchain
// helpers it needs. Kept under //go:build android so non-Android `go
// build ./...` does not require the dependency in go.mod.
import _ "golang.org/x/mobile/bind"
