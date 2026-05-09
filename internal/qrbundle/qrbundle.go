// Package qrbundle encodes an offline-CA-signed CBOR enrollment bundle as a
// QR-friendly base45 payload (RFC 9285) and decodes it back.
//
// The wire format is described in docs/qr-bundle.md. This package owns only
// the codec; rendering the base45 payload as a PNG QR is the operator
// tool's job (cmd/goat-bundle-qr), and verifying the resulting CBOR's
// Ed25519 signature against the pinned offline-CA root is internal/bundle's
// job. Mobile shells (Track C iOS, Track D Android) call Decode after
// their platform-native QR scanner produces a string.
package qrbundle

import (
	"errors"
	"fmt"
)

// QRAlphanumericLCeiling is the alphanumeric capacity of a QR Version 40
// code at error-correction level L (ISO/IEC 18004 Table 7). A base45
// payload longer than this cannot be carried in a single QR code; the
// operator must trim the bundle schema or use a non-QR delivery channel.
const QRAlphanumericLCeiling = 4296

// MaxBundleBytes is the largest binary input Encode will accept. It is
// chosen so the resulting base45 string fits in a QR Version 40 ECC L code
// in alphanumeric mode. base45 expands by 50% (2 bytes -> 3 chars), so the
// max is floor(QRAlphanumericLCeiling / 3) * 2 = 2864 bytes.
const MaxBundleBytes = QRAlphanumericLCeiling / 3 * 2

// ErrBundleTooLarge is returned by Encode when the input is too large to
// fit a single QR-40 / ECC L alphanumeric payload.
var ErrBundleTooLarge = errors.New("qrbundle: bundle exceeds single-QR capacity")

// ErrEmpty is returned by Encode and Decode when given a zero-length input.
var ErrEmpty = errors.New("qrbundle: empty input")

// Encode wraps `bundleBytes` (a CBOR-encoded offline-CA-signed bundle) in
// a base45 string suitable for QR alphanumeric-mode encoding or for
// pasting into an out-of-band channel for the end user to type/paste into
// the goat-client bundle-import dialog.
//
// Encode does not parse the CBOR or verify the signature — that is the
// caller's responsibility (typically internal/bundle, after Decode). It
// only enforces the QR sizing budget.
func Encode(bundleBytes []byte) (string, error) {
	if len(bundleBytes) == 0 {
		return "", ErrEmpty
	}
	if len(bundleBytes) > MaxBundleBytes {
		return "", fmt.Errorf("%w: %d bytes (max %d)", ErrBundleTooLarge, len(bundleBytes), MaxBundleBytes)
	}
	return base45Encode(bundleBytes), nil
}

// Decode reverses Encode. The returned bytes are the original
// `bundleBytes` and are ready to hand to internal/bundle for parse +
// signature verify.
//
// Decode rejects any character outside the base45 alphabet and any
// malformed length. It does not look at the CBOR contents.
func Decode(qrPayload string) ([]byte, error) {
	if len(qrPayload) == 0 {
		return nil, ErrEmpty
	}
	out, err := base45DecodeString(qrPayload)
	if err != nil {
		return nil, fmt.Errorf("qrbundle: %w", err)
	}
	return out, nil
}
