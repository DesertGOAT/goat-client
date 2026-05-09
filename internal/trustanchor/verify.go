package trustanchor

import (
	"crypto/ed25519"
	"errors"
	"fmt"
)

// ErrUntrusted is returned by Verify when no active anchor's public key
// validates the supplied signature.
var ErrUntrusted = errors.New("trustanchor: signature not signed by any active anchor")

// ErrNoActiveAnchors is returned by Verify when the Set contains no
// anchor whose validity window covers the current wall-clock — every
// pinned anchor has either not yet started or has already expired.
//
// Callers should treat this as a stale-binary signal: the daemon can
// neither sign nor accept bundles until it is upgraded.
var ErrNoActiveAnchors = errors.New("trustanchor: no active anchor at current time (binary is stale)")

// ErrSignatureSize is returned when the supplied signature is not the
// 64-byte length crypto/ed25519 produces.
var ErrSignatureSize = errors.New("trustanchor: signature wrong size")

// Verify checks bundleSig as an Ed25519 signature over bundleBytes
// against every anchor active at the current wall-clock. Returns the
// matching anchor on success, or one of [ErrNoActiveAnchors],
// [ErrSignatureSize], [ErrUntrusted].
//
// bundleBytes must be the canonical-CBOR signable payload — typically
// the output of internal/bundle.EnrollmentBundle.Signable. This package
// is intentionally agnostic to the bundle wire format so that a future
// signed-channel-update payload (ADR 0840 D5 v2) can reuse the same
// anchor set.
//
// Rotation behaviour: when the Set carries an old + new anchor whose
// windows overlap, both are tried in insertion order. The returned
// anchor identifies which key the bundle was actually signed under so
// the caller can record the active CA-id in audit logs.
func (s *Set) Verify(bundleSig []byte, bundleBytes []byte) (*Anchor, error) {
	if len(bundleSig) != ed25519.SignatureSize {
		return nil, fmt.Errorf("%w: got %d want %d", ErrSignatureSize, len(bundleSig), ed25519.SignatureSize)
	}
	now := s.now().UTC()
	hasActive := false
	for i := range s.anchors {
		a := &s.anchors[i]
		if !a.active(now) {
			continue
		}
		hasActive = true
		if ed25519.Verify(a.PublicKey, bundleBytes, bundleSig) {
			return a, nil
		}
	}
	if !hasActive {
		return nil, ErrNoActiveAnchors
	}
	return nil, ErrUntrusted
}
