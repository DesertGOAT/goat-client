// Package bundle is the GUI-side bundle preview helper.
//
// The full CBOR parse + Ed25519 signature verification against the pinned
// offline-CA root is owned by Track A (internal/tunnel + daemon). The GUI
// only needs a lightweight preview shape so the bundle-import dialog can
// display issued-to / site / expires / peer-pubkey / endpoints before the
// user clicks Apply. After Apply the daemon owns parse + verify
// authoritatively via the IPC ImportBundle method.
//
// Until Track A's parser is reachable from this package, Preview returns a
// stub Metadata so the dialog UX is exercisable end-to-end.
package bundle

import (
	"errors"
	"time"
)

// Metadata is the preview shape rendered by the bundle-import dialog.
//
// Mirrors the user-visible subset of the offline-CA-signed CBOR bundle
// schema documented in goat-trunk's docs/design/offline-enrollment.md.
type Metadata struct {
	IssuedTo   string
	Site       string
	NotBefore  time.Time
	NotAfter   time.Time
	PeerPubKey string
	Endpoints  []string
}

// Preview parses just enough of `raw` to populate a Metadata for display.
//
// Stub: returns a canned Metadata for any non-empty input. Real parsing
// lands when Track A's bundle package is wired in.
func Preview(raw []byte) (*Metadata, error) {
	if len(raw) == 0 {
		return nil, errors.New("bundle is empty")
	}
	now := time.Now().UTC()
	return &Metadata{
		IssuedTo:   "stub-device",
		Site:       "lab-stub",
		NotBefore:  now.Add(-time.Hour),
		NotAfter:   now.Add(90 * 24 * time.Hour),
		PeerPubKey: "STUB+wg-cp0+peer+pubkey+base64==",
		Endpoints:  []string{"wg-cp0.example.invalid:51821"},
	}, nil
}
