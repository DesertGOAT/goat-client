//go:build android

package goatclient

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// SDK-side half of the Android socket-protection bridge.
//
// The Go engine's outer wg-cp0 UDP socket must NOT loop back through
// the VPN tunnel — otherwise it tries to encrypt its own underlay
// traffic. android.net.VpnService.protect(fd) marks a socket as
// underlay-routed; we bridge that across the gomobile boundary by
// stashing the Kotlin-supplied callback here. internal/net (Track A)
// will read this back via a getter on the same package or by importing
// this SDK at a build-tag-narrowed seam.
//
// Forked from netbird client/net/protectsocket_android.go (BSD-3-Clause)
// — kept inline in the SDK package to avoid a circular Track A
// dependency during the converge window. When Track A's internal/net
// package is in place, this can move (and the SDK will re-export the
// setter for backwards compat).
var (
	androidProtectSocketLock sync.Mutex
	androidProtectSocket     func(fd int32) bool
)

func setAndroidProtectSocketFn(fn func(fd int32) bool) {
	androidProtectSocketLock.Lock()
	androidProtectSocket = fn
	androidProtectSocketLock.Unlock()
}

// AndroidProtectSocket is exported so internal/net (Track A) can read
// the live callback without re-implementing the lock dance. Returns
// false if no callback has been set yet (engine starting before
// NewClient — should not happen in practice).
func AndroidProtectSocket(fd int32) bool {
	androidProtectSocketLock.Lock()
	fn := androidProtectSocket
	androidProtectSocketLock.Unlock()
	if fn == nil {
		return false
	}
	return fn(fd)
}

func bundleChecksum(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
