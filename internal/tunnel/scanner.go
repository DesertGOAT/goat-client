package tunnel

import (
	"bufio"
	"io"
)

// newLineScanner returns a bufio.Scanner with a 1 MiB buffer — UAPI
// responses for a single-peer tunnel are well under 1 KiB, but the
// generous buffer keeps Stats reads correct if a future change inflates
// the output (e.g. per-allowed-ip stats).
func newLineScanner(r io.Reader) *bufio.Scanner {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64<<10), 1<<20)
	return s
}
