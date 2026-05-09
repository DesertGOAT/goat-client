package ipc

import (
	"runtime"
)

// DefaultAddr returns the platform-conventional daemon address.
//
//   - Linux/macOS: a Unix domain socket under /var/run.
//   - Windows: a named pipe under \\.\pipe\.
//
// Track A wires the real transport. Until then the stub Client ignores the
// address entirely.
func DefaultAddr() string {
	if runtime.GOOS == "windows" {
		return `\\.\pipe\goat-client`
	}
	return "unix:///var/run/goat-client.sock"
}

// NewClient returns the Client implementation appropriate for `addr`.
//
// Today this always returns the in-process stub: real JSON-RPC dialing is
// Track A's deliverable. Once Track A lands, this function selects between
// stub and real transport based on the address scheme.
func NewClient(addr string) (Client, error) {
	return newStubClient(addr), nil
}
