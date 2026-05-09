// Package trustanchor pins the offline-CA Ed25519 root public keys that
// goat-client will accept as bundle signers.
//
// Per ADR 0840 D5 (DesertBreadBird@docs/adr/0840-…), the trust anchor is
// not user-configurable. The active set is compiled into the binary at
// build time from anchors.yaml so a tampered config file cannot persuade
// the daemon to accept a malicious offline-CA root.
//
// Multiple anchors may be active simultaneously during CA-rotation
// windows (ADR 0406 / Block 79). Verify tries each anchor whose
// [valid_from, valid_until] window contains the current wall-clock and
// returns the first match. Stale binaries whose anchors have all expired
// fail closed with [ErrNoActiveAnchors] — the daemon surfaces a "please
// update goat-client" prompt rather than accepting an unverified bundle.
//
// The on-disk authoritative source is goat-trunk
// ops/enrollment/public-keys/. anchors.yaml in this package is the build-
// time copy; refresh it with the cmd/anchorgen tool when the offline CA
// publishes a new public key.
package trustanchor

//go:generate go run ../../cmd/anchorgen -in anchors.yaml -out embedded.go
