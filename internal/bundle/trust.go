package bundle

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// ErrNoTrustRoots is returned when verification is attempted with an empty
// TrustRoots set — fail-closed so a misconfigured daemon never accepts an
// unsigned bundle.
var ErrNoTrustRoots = errors.New("no trust roots configured")

// ErrUntrustedBundle is returned when the bundle's signature does not match
// any of the configured trust roots.
var ErrUntrustedBundle = errors.New("bundle signature does not match any trust root")

// TrustRoots is the set of Ed25519 public keys that a daemon will accept as
// signers of inbound bundles. A daemon typically loads exactly one root —
// the offline CA's bundle-signing pubkey — but the slice form supports CA
// rotation: during a rotation window both old and new roots are pinned,
// and bundles signed by either are accepted.
//
// Wire format on disk: PEM-encoded SubjectPublicKeyInfo blocks
// (`-----BEGIN PUBLIC KEY-----`), one block per root, concatenated.
type TrustRoots struct {
	keys []ed25519.PublicKey
}

// NewTrustRoots constructs a TrustRoots set from raw Ed25519 public keys.
func NewTrustRoots(keys ...ed25519.PublicKey) (*TrustRoots, error) {
	tr := &TrustRoots{}
	for i, k := range keys {
		if len(k) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("trust root %d: wrong key size: got %d want %d", i, len(k), ed25519.PublicKeySize)
		}
		tr.keys = append(tr.keys, append(ed25519.PublicKey(nil), k...))
	}
	return tr, nil
}

// LoadTrustRootsFromFile reads a PEM file containing one or more
// `PUBLIC KEY` blocks and returns the TrustRoots they encode. Each block
// must be an Ed25519 SubjectPublicKeyInfo.
func LoadTrustRootsFromFile(path string) (*TrustRoots, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read trust roots: %w", err)
	}
	return LoadTrustRootsFromPEM(data)
}

// LoadTrustRootsFromPEM parses one or more `PUBLIC KEY` PEM blocks.
func LoadTrustRootsFromPEM(data []byte) (*TrustRoots, error) {
	var keys []ed25519.PublicKey
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "PUBLIC KEY" {
			continue
		}
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse trust root: %w", err)
		}
		ed, ok := pub.(ed25519.PublicKey)
		if !ok {
			return nil, fmt.Errorf("trust root is not an Ed25519 key: %T", pub)
		}
		keys = append(keys, ed)
	}
	if len(keys) == 0 {
		return nil, errors.New("no PUBLIC KEY blocks found")
	}
	return &TrustRoots{keys: keys}, nil
}

// Empty reports whether the trust set has no keys.
func (tr *TrustRoots) Empty() bool {
	return tr == nil || len(tr.keys) == 0
}

// VerifyBundle checks the bundle's signature against every pinned root and
// returns nil on first match. Returns ErrNoTrustRoots if the set is empty
// and ErrUntrustedBundle if no root matches.
func (tr *TrustRoots) VerifyBundle(b *EnrollmentBundle) error {
	if tr.Empty() {
		return ErrNoTrustRoots
	}
	var lastErr error
	for _, k := range tr.keys {
		if err := b.Verify(k); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr != nil {
		return fmt.Errorf("%w: %v", ErrUntrustedBundle, lastErr)
	}
	return ErrUntrustedBundle
}
