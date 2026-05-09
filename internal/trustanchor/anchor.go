package trustanchor

import (
	"crypto/ed25519"
	"fmt"
	"sort"
	"time"
)

// Anchor is one pinned offline-CA root public key with its validity window.
//
// PublicKey is the raw 32-byte Ed25519 public half (the same shape
// crypto/ed25519.Verify expects). Name is the human-readable label that
// appears in audit logs ("dev-desertbread-ca-2026-04-20"). Issuer is the
// CA-identity string the offline-CA stamps into bundle.CAID — kept here
// so a successful Verify can be cross-checked against bundle.CAID by the
// caller without re-deriving the issuer from the key bytes.
type Anchor struct {
	Name       string
	Issuer     string
	PublicKey  ed25519.PublicKey
	ValidFrom  time.Time
	ValidUntil time.Time
}

// active reports whether at is within [ValidFrom, ValidUntil] inclusive.
// Comparison is done in UTC — the YAML descriptor stores all dates in
// UTC, so wall-clock skew on the device side is the only source of
// drift.
func (a *Anchor) active(at time.Time) bool {
	at = at.UTC()
	if at.Before(a.ValidFrom.UTC()) {
		return false
	}
	if at.After(a.ValidUntil.UTC()) {
		return false
	}
	return true
}

// Set is the collection of anchors compiled into the binary. Construct
// the production set with [Default]; tests construct synthetic sets with
// [NewSet].
//
// Set is immutable after construction. Verify is safe for concurrent
// use.
type Set struct {
	anchors []Anchor
	now     func() time.Time
}

// NewSet returns a Set containing the supplied anchors. Returns an
// error if any anchor has the wrong key size, an empty Name, or a
// ValidUntil that does not strictly follow ValidFrom — caller bugs that
// would otherwise silently produce a Set that can never verify.
//
// The order of anchors is preserved; Verify iterates in insertion order.
func NewSet(anchors ...Anchor) (*Set, error) {
	for i, a := range anchors {
		if a.Name == "" {
			return nil, fmt.Errorf("anchor %d: name is empty", i)
		}
		if len(a.PublicKey) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("anchor %q: public key wrong size: got %d want %d",
				a.Name, len(a.PublicKey), ed25519.PublicKeySize)
		}
		if !a.ValidUntil.After(a.ValidFrom) {
			return nil, fmt.Errorf("anchor %q: valid_until (%s) must be strictly after valid_from (%s)",
				a.Name,
				a.ValidUntil.UTC().Format(time.RFC3339),
				a.ValidFrom.UTC().Format(time.RFC3339))
		}
	}
	return &Set{
		anchors: append([]Anchor(nil), anchors...),
		now:     time.Now,
	}, nil
}

// Default returns the build-time-embedded Set of pinned anchors.
//
// Panics if the embedded data is malformed — that condition can only
// arise from a corrupted code-generator run, which would also fail the
// build tests that exercise this constructor. Callers do not need to
// guard against the panic at runtime.
func Default() *Set {
	s, err := NewSet(embedded...)
	if err != nil {
		panic(fmt.Sprintf("trustanchor: embedded anchors malformed: %v", err))
	}
	return s
}

// withClock returns a copy of s whose Verify uses fn instead of
// time.Now. Test-only.
func (s *Set) withClock(fn func() time.Time) *Set {
	cp := *s
	cp.now = fn
	return &cp
}

// All returns a copy of every anchor in the set, regardless of validity.
// Useful for diagnostics ("which anchors does this binary carry?").
func (s *Set) All() []Anchor {
	return append([]Anchor(nil), s.anchors...)
}

// Active returns the anchors whose [ValidFrom, ValidUntil] window
// contains at, sorted by ValidFrom ascending so the oldest still-valid
// anchor appears first. The returned slice is a fresh copy.
func (s *Set) Active(at time.Time) []Anchor {
	var out []Anchor
	for _, a := range s.anchors {
		if a.active(at) {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ValidFrom.Before(out[j].ValidFrom)
	})
	return out
}
