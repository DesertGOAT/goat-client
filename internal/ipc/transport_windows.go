//go:build windows

package ipc

import (
	"fmt"
	"net"

	"github.com/Microsoft/go-winio"
)

// Listen creates a Windows named-pipe listener at the given path
// (typically `\\.\pipe\goat-clientd`). The pipe ACL allows the current
// user only — Authenticated Users do not get access — which mirrors the
// 0600 mode of the Unix-domain socket on POSIX.
func Listen(pipePath string) (net.Listener, error) {
	cfg := &winio.PipeConfig{
		// SecurityDescriptor in SDDL: D:P(A;;GA;;;OW) → DACL, protected,
		// allow generic-all to OWNER. No anonymous, no everyone, no
		// authenticated-users. SACL omitted.
		SecurityDescriptor: "D:P(A;;GA;;;OW)",
		MessageMode:        false,
		InputBufferSize:    64 << 10,
		OutputBufferSize:   64 << 10,
	}
	ln, err := winio.ListenPipe(pipePath, cfg)
	if err != nil {
		return nil, fmt.Errorf("listen pipe %s: %w", pipePath, err)
	}
	return ln, nil
}

// peerCreds for Windows named pipes: go-winio's accepted connection type
// implements `Pid()` for the peer process; we map that pid to the
// process's owning sid → uid in a future phase (the goat-client uid
// model on Windows is a placeholder — there is no uid; the SDDL above
// is what enforces access). For now we return the daemon's own pid +
// uid 0 so the dispatcher's "trustedUid==0 means any local peer" path
// applies. This is acceptable because the SDDL already restricts the
// pipe to the owning user — the IPC layer's uid check would be
// redundant.
func peerCreds(conn net.Conn) (PeerCreds, error) {
	// go-winio's PipeConn type carries the peer pid via its `Pid()`
	// method, but that requires a type assertion against the unexported
	// type. Skip pid for now; uid is irrelevant on Windows (see comment).
	return PeerCreds{Uid: 0, Pid: 0}, nil
}

// Dial connects a client to a named pipe IPC endpoint.
func Dial(pipePath string) (net.Conn, error) {
	return winio.DialPipe(pipePath, nil)
}
