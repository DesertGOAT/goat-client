package qrbundle

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"
)

func TestEncodeDecode_RoundTrip(t *testing.T) {
	sizes := []int{1, 2, 3, 16, 100, 511, 512, 1500, 1680, 2234, MaxBundleBytes}
	for _, n := range sizes {
		buf := make([]byte, n)
		if _, err := rand.Read(buf); err != nil {
			t.Fatalf("rand.Read: %v", err)
		}
		s, err := Encode(buf)
		if err != nil {
			t.Fatalf("size %d: Encode: %v", n, err)
		}
		got, err := Decode(s)
		if err != nil {
			t.Fatalf("size %d: Decode: %v", n, err)
		}
		if !bytes.Equal(got, buf) {
			t.Fatalf("size %d: round-trip mismatch", n)
		}
	}
}

func TestEncode_RejectsEmpty(t *testing.T) {
	if _, err := Encode(nil); !errors.Is(err, ErrEmpty) {
		t.Errorf("Encode(nil) = %v, want ErrEmpty", err)
	}
	if _, err := Encode([]byte{}); !errors.Is(err, ErrEmpty) {
		t.Errorf("Encode([]) = %v, want ErrEmpty", err)
	}
}

func TestDecode_RejectsEmpty(t *testing.T) {
	if _, err := Decode(""); !errors.Is(err, ErrEmpty) {
		t.Errorf("Decode(\"\") = %v, want ErrEmpty", err)
	}
}

// MaxBundleBytes is the QR-40/L ceiling. One byte over must be rejected,
// otherwise the operator would generate a payload that no QR scanner can
// fit.
func TestEncode_RejectsAboveQR40Ceiling(t *testing.T) {
	buf := make([]byte, MaxBundleBytes+1)
	if _, err := Encode(buf); !errors.Is(err, ErrBundleTooLarge) {
		t.Errorf("Encode(MaxBundleBytes+1) = %v, want ErrBundleTooLarge", err)
	}
}

// At the ceiling exactly, Encode must succeed and the resulting base45
// string must fit a QR-40/L alphanumeric region (4296 chars).
func TestEncode_AtQR40Ceiling(t *testing.T) {
	buf := make([]byte, MaxBundleBytes)
	s, err := Encode(buf)
	if err != nil {
		t.Fatalf("Encode at ceiling: %v", err)
	}
	if len(s) > QRAlphanumericLCeiling {
		t.Errorf("encoded len %d > QR-40/L alphanumeric ceiling %d", len(s), QRAlphanumericLCeiling)
	}
}

// Sizing-budget regression tests. These document the QR-version-vs-payload
// boundaries called out in docs/qr-bundle.md so a future bundle-schema
// change that pushes past one of them surfaces as a test signal, not a
// scanner failure in the field.
func TestEncode_QRVersionBoundaries(t *testing.T) {
	// ISO/IEC 18004 Table 7, ECC L, alphanumeric-mode capacity.
	const (
		qr25LAlpha = 2520
		qr30LAlpha = 3351
	)
	cases := []struct {
		name    string
		size    int
		fitsQR  int
		maxChar int
	}{
		{"typical 1500 B bundle fits QR-25", 1500, 25, qr25LAlpha},
		{"2 kB bundle fits QR-30", 2000, 30, qr30LAlpha},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]byte, tc.size)
			s, err := Encode(buf)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if len(s) > tc.maxChar {
				t.Errorf("size %d encoded to %d chars, exceeds QR-%d/L capacity %d",
					tc.size, len(s), tc.fitsQR, tc.maxChar)
			}
		})
	}
}

func TestDecode_RejectsCorruptInput(t *testing.T) {
	buf := make([]byte, 64)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	s, err := Encode(buf)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// Replace one char with something not in the alphabet.
	corrupt := []byte(s)
	corrupt[3] = '!'
	if _, err := Decode(string(corrupt)); err == nil {
		t.Errorf("Decode of corrupted input should fail, got nil")
	}
}
