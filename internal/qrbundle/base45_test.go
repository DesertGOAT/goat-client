package qrbundle

import (
	"errors"
	"strings"
	"testing"
)

// RFC 9285 §4.4 — canonical encoding vectors. If any of these break, the
// codec is no longer base45 compatible and mobile decoders will reject.
func TestBase45_RFC9285Vectors(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"AB", "BB8"},
		{"Hello!!", "%69 VD92EX0"},
		{"base-45", "UJCLQE7W581"},
		{"ietf!", "QED8WEX0"},
	}
	for _, tc := range cases {
		got := base45Encode([]byte(tc.in))
		if got != tc.want {
			t.Errorf("encode(%q) = %q, want %q", tc.in, got, tc.want)
		}
		round, err := base45DecodeString(tc.want)
		if err != nil {
			t.Errorf("decode(%q) returned error: %v", tc.want, err)
			continue
		}
		if string(round) != tc.in {
			t.Errorf("decode(%q) = %q, want %q", tc.want, round, tc.in)
		}
	}
}

func TestBase45_EncodedLen(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{0, 0},
		{1, 2},
		{2, 3},
		{3, 5},
		{4, 6},
		{5, 8},
		{1500, 2250},
		{1501, 2252},
	}
	for _, tc := range cases {
		got := base45EncodedLen(tc.in)
		if got != tc.want {
			t.Errorf("base45EncodedLen(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestBase45_DecodeRejectsBadLength(t *testing.T) {
	// length % 3 == 1 is unreachable from any valid encode.
	_, err := base45DecodeString("A")
	if !errors.Is(err, errBase45) {
		t.Errorf("expected errBase45 for length 1, got %v", err)
	}
	_, err = base45DecodeString("BB81")
	if !errors.Is(err, errBase45) {
		t.Errorf("expected errBase45 for length 4, got %v", err)
	}
}

func TestBase45_DecodeRejectsBadChars(t *testing.T) {
	// 'a' (lowercase) is not in the alphabet.
	_, err := base45DecodeString("abc")
	if !errors.Is(err, errBase45) {
		t.Errorf("expected errBase45 for lowercase, got %v", err)
	}
	// '!' is not in the alphabet either.
	_, err = base45DecodeString("BB!")
	if !errors.Is(err, errBase45) {
		t.Errorf("expected errBase45 for '!', got %v", err)
	}
}

func TestBase45_DecodeRejectsOversizeTriplet(t *testing.T) {
	// Largest valid triplet decodes to 0xFFFF. Any triplet whose value
	// exceeds 65535 must be rejected — otherwise a malicious payload
	// could smuggle extra bits past the codec.
	//
	// alphabet index 44 = ':'. Triplet ":::" decodes to
	// 44 + 44*45 + 44*45*45 = 91124, well above 0xFFFF.
	_, err := base45DecodeString(":::")
	if !errors.Is(err, errBase45) {
		t.Errorf("expected errBase45 for oversize triplet, got %v", err)
	}
}

func TestBase45_DecodeRejectsOversizeTrailingPair(t *testing.T) {
	// Trailing pair must decode to 0..255. "::" = 44 + 44*45 = 2024.
	_, err := base45DecodeString("BB8::")
	if !errors.Is(err, errBase45) {
		t.Errorf("expected errBase45 for oversize trailing pair, got %v", err)
	}
}

func TestBase45_AlphabetIsAllQRSafe(t *testing.T) {
	// The whole point of base45 is that every output character is in QR
	// alphanumeric mode. Catch a typo in the alphabet constant by
	// asserting length and uniqueness.
	if len(base45Alphabet) != 45 {
		t.Fatalf("alphabet length = %d, want 45", len(base45Alphabet))
	}
	seen := map[byte]bool{}
	for i := 0; i < len(base45Alphabet); i++ {
		c := base45Alphabet[i]
		if seen[c] {
			t.Errorf("duplicate char %q in alphabet", c)
		}
		seen[c] = true
	}
}

func TestBase45_RoundTripBytePatterns(t *testing.T) {
	// Single bytes 0..255 round-trip through 2-char output.
	for b := 0; b < 256; b++ {
		in := []byte{byte(b)}
		s := base45Encode(in)
		if len(s) != 2 {
			t.Fatalf("byte %d: encoded length = %d, want 2", b, len(s))
		}
		out, err := base45DecodeString(s)
		if err != nil {
			t.Fatalf("byte %d: decode error: %v", b, err)
		}
		if len(out) != 1 || out[0] != byte(b) {
			t.Fatalf("byte %d: round-trip = %v", b, out)
		}
	}
}

func TestBase45_AllQRAlphanumericChars(t *testing.T) {
	// Every byte produced by base45Encode must be a member of the QR
	// alphanumeric set. We assert this by re-checking the alphabet.
	in := strings.Repeat("\xff\x00\x55\xaa", 64)
	s := base45Encode([]byte(in))
	for i := 0; i < len(s); i++ {
		if base45Decode[s[i]] < 0 {
			t.Errorf("encoded char %q at offset %d not in alphabet", s[i], i)
		}
	}
}
