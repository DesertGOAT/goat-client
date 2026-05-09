package bundle

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"
	"time"
)

func newTestBundle(t *testing.T) (*EnrollmentBundle, ed25519.PrivateKey, ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	b := &EnrollmentBundle{
		Version:            Version,
		DeviceID:           "test-device",
		PeerPubkey:         []byte("0123456789abcdef0123456789abcdef"),
		ACLGroups:          []string{"workstations"},
		Site:               "kwt-aj-A",
		KnownEndpoints:     []KnownEndpoint{{Addr: "10.0.0.1:51820", Pubkey: []byte("relaypubkey00000000000000000000aa"), Kind: KindRelay, MeshAddr: "198.18.0.1"}},
		IssuedAt:           now,
		ActivationDeadline: now.Add(72 * time.Hour),
		ExpiresAt:          now.Add(365 * 24 * time.Hour),
		Nonce:              []byte("nonce-bytes-1234"),
		CAID:               "offline-ca-2026-05",
	}
	if err := b.Sign(func(payload []byte) ([]byte, error) {
		return ed25519.Sign(priv, payload), nil
	}); err != nil {
		t.Fatalf("sign: %v", err)
	}
	return b, priv, pub
}

// Sign produces a signature over the canonical payload and stores it on the
// bundle. Test-only helper — production signing happens in the offline-CA
// host workflow (goat-trunk ops/enrollment), not in the daemon.
func (b *EnrollmentBundle) Sign(sign func([]byte) ([]byte, error)) error {
	payload, err := b.Signable()
	if err != nil {
		return err
	}
	sig, err := sign(payload)
	if err != nil {
		return err
	}
	if len(sig) != ed25519.SignatureSize {
		return errors.New("signature wrong size")
	}
	b.Signature = sig
	return nil
}

func TestRoundTripVerify(t *testing.T) {
	b, _, pub := newTestBundle(t)
	wire, err := b.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := Unmarshal(wire)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.DeviceID != b.DeviceID {
		t.Errorf("DeviceID round-trip: got %q want %q", got.DeviceID, b.DeviceID)
	}
	if err := got.Verify(pub); err != nil {
		t.Errorf("Verify: %v", err)
	}
}

func TestVerifyRejectsTamperedPayload(t *testing.T) {
	b, _, pub := newTestBundle(t)
	b.DeviceID = "evil-device"
	if err := b.Verify(pub); err == nil {
		t.Fatal("Verify accepted tampered bundle")
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	b, _, _ := newTestBundle(t)
	other, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519: %v", err)
	}
	if err := b.Verify(other); err == nil {
		t.Fatal("Verify accepted bundle with wrong key")
	}
}

func TestCheckExpiry(t *testing.T) {
	b := &EnrollmentBundle{ExpiresAt: time.Now().Add(-time.Hour)}
	if err := b.CheckExpiry(time.Now()); !errors.Is(err, ErrExpired) {
		t.Errorf("CheckExpiry: want ErrExpired, got %v", err)
	}
	b.ExpiresAt = time.Now().Add(time.Hour)
	if err := b.CheckExpiry(time.Now()); err != nil {
		t.Errorf("CheckExpiry on fresh bundle: %v", err)
	}
}

func TestCheckCPDeviceKeypairUnpaired(t *testing.T) {
	b := &EnrollmentBundle{CPDevicePubkey: []byte("only-pub")}
	if err := b.CheckCPDeviceKeypair(); !errors.Is(err, ErrCPDeviceKeypairUnpaired) {
		t.Errorf("CheckCPDeviceKeypair: want ErrCPDeviceKeypairUnpaired, got %v", err)
	}
}

func TestTrustRootsVerify(t *testing.T) {
	b, _, pub := newTestBundle(t)
	tr, err := NewTrustRoots(pub)
	if err != nil {
		t.Fatalf("NewTrustRoots: %v", err)
	}
	if err := tr.VerifyBundle(b); err != nil {
		t.Errorf("VerifyBundle (matching root): %v", err)
	}
	other, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519: %v", err)
	}
	tr2, _ := NewTrustRoots(other)
	if err := tr2.VerifyBundle(b); !errors.Is(err, ErrUntrustedBundle) {
		t.Errorf("VerifyBundle (non-matching root): want ErrUntrustedBundle, got %v", err)
	}
	empty := &TrustRoots{}
	if err := empty.VerifyBundle(b); !errors.Is(err, ErrNoTrustRoots) {
		t.Errorf("VerifyBundle (empty): want ErrNoTrustRoots, got %v", err)
	}
}

func TestLoadTrustRootsFromPEM(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("marshal pkix: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	tr, err := LoadTrustRootsFromPEM(pemBytes)
	if err != nil {
		t.Fatalf("LoadTrustRootsFromPEM: %v", err)
	}
	if tr.Empty() {
		t.Fatal("trust roots empty after load")
	}
}
