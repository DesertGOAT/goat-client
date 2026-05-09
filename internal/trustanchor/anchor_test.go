package trustanchor

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"
)

// fixedClock returns a closure suitable for Set.withClock that always
// reports the same wall-clock value.
func fixedClock(at time.Time) func() time.Time {
	return func() time.Time { return at }
}

// genAnchor produces an Anchor whose validity window contains `at`. The
// caller can pass a custom clock when constructing the Set so signing
// and verification observe the same notion of "now".
func genAnchor(t *testing.T, name string, validFrom, validUntil time.Time) (Anchor, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	return Anchor{
		Name:       name,
		Issuer:     name,
		PublicKey:  pub,
		ValidFrom:  validFrom,
		ValidUntil: validUntil,
	}, priv
}

func TestNewSetRejectsBadAnchors(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		anchor  Anchor
		wantErr string
	}{
		{
			name: "empty name",
			anchor: Anchor{
				PublicKey:  make([]byte, ed25519.PublicKeySize),
				ValidFrom:  now,
				ValidUntil: now.Add(time.Hour),
			},
			wantErr: "name is empty",
		},
		{
			name: "wrong key size",
			anchor: Anchor{
				Name:       "x",
				PublicKey:  []byte{0x00},
				ValidFrom:  now,
				ValidUntil: now.Add(time.Hour),
			},
			wantErr: "wrong size",
		},
		{
			name: "until before from",
			anchor: Anchor{
				Name:       "x",
				PublicKey:  make([]byte, ed25519.PublicKeySize),
				ValidFrom:  now,
				ValidUntil: now.Add(-time.Hour),
			},
			wantErr: "valid_until",
		},
		{
			name: "until equals from",
			anchor: Anchor{
				Name:       "x",
				PublicKey:  make([]byte, ed25519.PublicKeySize),
				ValidFrom:  now,
				ValidUntil: now,
			},
			wantErr: "valid_until",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewSet(tc.anchor)
			if err == nil {
				t.Fatalf("NewSet: want error containing %q, got nil", tc.wantErr)
			}
			if !contains(err.Error(), tc.wantErr) {
				t.Fatalf("NewSet err = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestVerifySingleActiveAnchor(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	a, priv := genAnchor(t, "ca-1", now.Add(-24*time.Hour), now.Add(24*time.Hour))
	s, err := NewSet(a)
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	s = s.withClock(fixedClock(now))

	payload := []byte("synthetic-bundle-payload")
	sig := ed25519.Sign(priv, payload)

	got, err := s.Verify(sig, payload)
	if err != nil {
		t.Fatalf("Verify: unexpected error %v", err)
	}
	if got.Name != "ca-1" {
		t.Errorf("Verify returned anchor %q, want %q", got.Name, "ca-1")
	}
}

func TestVerifyRotationAcceptsBoth(t *testing.T) {
	// Rotation window: old anchor's validity overlaps with the new
	// anchor's. Bundles signed by either key must verify, and Verify
	// must report the actual signing anchor (not just the first one
	// tried) so audit logs are correct.
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	old, oldPriv := genAnchor(t, "ca-old", now.Add(-30*24*time.Hour), now.Add(7*24*time.Hour))
	newer, newPriv := genAnchor(t, "ca-new", now.Add(-1*24*time.Hour), now.Add(60*24*time.Hour))

	s, err := NewSet(old, newer)
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	s = s.withClock(fixedClock(now))

	payload := []byte("rotation-bundle-payload")

	t.Run("signed by old", func(t *testing.T) {
		sig := ed25519.Sign(oldPriv, payload)
		got, err := s.Verify(sig, payload)
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if got.Name != "ca-old" {
			t.Errorf("anchor = %q, want ca-old", got.Name)
		}
	})

	t.Run("signed by new", func(t *testing.T) {
		sig := ed25519.Sign(newPriv, payload)
		got, err := s.Verify(sig, payload)
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if got.Name != "ca-new" {
			t.Errorf("anchor = %q, want ca-new", got.Name)
		}
	})
}

func TestVerifyRefusesExpiredAnchor(t *testing.T) {
	// Anchor whose validity ended before "now". The signature is
	// genuine, but the binary must refuse it because the anchor is
	// retired. With no other active anchor the error must be
	// ErrNoActiveAnchors so callers can distinguish a stale binary
	// from an actually-bad signature.
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	a, priv := genAnchor(t, "ca-expired", now.Add(-90*24*time.Hour), now.Add(-1*24*time.Hour))
	s, err := NewSet(a)
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	s = s.withClock(fixedClock(now))

	payload := []byte("expired-bundle-payload")
	sig := ed25519.Sign(priv, payload)

	got, err := s.Verify(sig, payload)
	if got != nil {
		t.Fatalf("Verify returned anchor %q, want nil", got.Name)
	}
	if !errors.Is(err, ErrNoActiveAnchors) {
		t.Fatalf("Verify err = %v, want ErrNoActiveAnchors", err)
	}
}

func TestVerifyRefusesNotYetValidAnchor(t *testing.T) {
	// Anchor's ValidFrom is in the future. Same fail-closed as expired.
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	a, priv := genAnchor(t, "ca-future", now.Add(7*24*time.Hour), now.Add(60*24*time.Hour))
	s, err := NewSet(a)
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	s = s.withClock(fixedClock(now))

	payload := []byte("future-bundle-payload")
	sig := ed25519.Sign(priv, payload)

	if _, err := s.Verify(sig, payload); !errors.Is(err, ErrNoActiveAnchors) {
		t.Fatalf("Verify err = %v, want ErrNoActiveAnchors", err)
	}
}

func TestVerifyRotationKeepsActiveWhenOneIsExpired(t *testing.T) {
	// Old anchor has retired but new anchor is still active.
	// Bundles signed by the still-active anchor must verify;
	// bundles signed by the retired anchor must be refused with
	// ErrUntrusted (there *is* an active anchor, but this signature
	// doesn't match it — distinct from ErrNoActiveAnchors).
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	old, oldPriv := genAnchor(t, "ca-old", now.Add(-90*24*time.Hour), now.Add(-1*24*time.Hour))
	newer, newPriv := genAnchor(t, "ca-new", now.Add(-1*24*time.Hour), now.Add(60*24*time.Hour))

	s, err := NewSet(old, newer)
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	s = s.withClock(fixedClock(now))

	payload := []byte("mixed-rotation-payload")

	gotNew, err := s.Verify(ed25519.Sign(newPriv, payload), payload)
	if err != nil {
		t.Fatalf("Verify(new): %v", err)
	}
	if gotNew.Name != "ca-new" {
		t.Errorf("active signer = %q, want ca-new", gotNew.Name)
	}

	if _, err := s.Verify(ed25519.Sign(oldPriv, payload), payload); !errors.Is(err, ErrUntrusted) {
		t.Fatalf("Verify(old) err = %v, want ErrUntrusted", err)
	}
}

func TestVerifyRefusesUnknownSigner(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	a, _ := genAnchor(t, "ca-1", now.Add(-time.Hour), now.Add(time.Hour))
	s, err := NewSet(a)
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	s = s.withClock(fixedClock(now))

	_, attackerPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	payload := []byte("attacker-bundle-payload")
	sig := ed25519.Sign(attackerPriv, payload)

	if _, err := s.Verify(sig, payload); !errors.Is(err, ErrUntrusted) {
		t.Fatalf("Verify err = %v, want ErrUntrusted", err)
	}
}

func TestVerifyRefusesShortSignature(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	a, _ := genAnchor(t, "ca-1", now.Add(-time.Hour), now.Add(time.Hour))
	s, _ := NewSet(a)
	s = s.withClock(fixedClock(now))

	if _, err := s.Verify([]byte{0x00, 0x01}, []byte("payload")); !errors.Is(err, ErrSignatureSize) {
		t.Fatalf("Verify err = %v, want ErrSignatureSize", err)
	}
}

func TestActiveSorted(t *testing.T) {
	// Active() must return anchors sorted by ValidFrom ascending so
	// callers iterating the result get a stable, oldest-first order.
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	a1, _ := genAnchor(t, "ca-newer", now.Add(-1*24*time.Hour), now.Add(60*24*time.Hour))
	a2, _ := genAnchor(t, "ca-older", now.Add(-30*24*time.Hour), now.Add(7*24*time.Hour))

	s, err := NewSet(a1, a2)
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	got := s.Active(now)
	if len(got) != 2 {
		t.Fatalf("Active len = %d, want 2", len(got))
	}
	if got[0].Name != "ca-older" || got[1].Name != "ca-newer" {
		t.Errorf("Active order = [%s, %s], want [ca-older, ca-newer]", got[0].Name, got[1].Name)
	}
}

func TestDefaultEmbeddedSetParses(t *testing.T) {
	// Sanity check that the build-time-embedded anchors are well-
	// formed. If anchorgen ever emits a malformed Anchor — wrong
	// key length, zero name, inverted dates — Default() panics; this
	// surfaces it as a test failure rather than a daemon crash.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Default() panicked: %v", r)
		}
	}()
	s := Default()
	if len(s.All()) == 0 {
		t.Fatal("Default() returned empty set; expected at least one embedded anchor")
	}
	for _, a := range s.All() {
		if len(a.PublicKey) != ed25519.PublicKeySize {
			t.Errorf("anchor %q: pubkey size %d, want %d", a.Name, len(a.PublicKey), ed25519.PublicKeySize)
		}
		if a.Name == "" {
			t.Error("embedded anchor with empty name")
		}
		if !a.ValidUntil.After(a.ValidFrom) {
			t.Errorf("anchor %q: valid_until not after valid_from", a.Name)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
