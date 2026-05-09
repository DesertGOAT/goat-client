//go:build unix

package ipc

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// Listen creates a Unix-domain-socket listener at the given path. Removes
// any stale socket file and sets mode 0600 so only the owning uid can
// connect (the kernel enforces this on connect).
func Listen(socketPath string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
		return nil, fmt.Errorf("mkdir socket dir: %w", err)
	}
	// Best-effort cleanup of a stale socket. If a different process is
	// listening, Listen below will fail with EADDRINUSE — we don't try to
	// distinguish stale-vs-live here; that's the operator's job.
	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("clean stale socket: %w", err)
	}
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("listen unix: %w", err)
	}
	if err := os.Chmod(socketPath, 0o600); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("chmod socket: %w", err)
	}
	return ln, nil
}

// peerCreds extracts the uid + pid from the peer end of a Unix socket
// connection. Per-OS impl in transport_unix_linux.go / transport_unix_darwin.go.
func peerCreds(conn net.Conn) (PeerCreds, error) {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return PeerCreds{}, fmt.Errorf("not a unix conn: %T", conn)
	}
	return peerCredsUnix(uc)
}

// Dial connects to a Unix-domain-socket IPC endpoint. Used by the GUI
// (Fyne) and by the integration test client.
func Dial(socketPath string) (net.Conn, error) {
	return net.Dial("unix", socketPath)
}
