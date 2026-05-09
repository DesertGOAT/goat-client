//go:build linux

package ipc

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// peerCredsUnix reads SO_PEERCRED from a Linux unix socket. Returns
// (uid, pid) of the peer process at connect time.
func peerCredsUnix(uc *net.UnixConn) (PeerCreds, error) {
	raw, err := uc.SyscallConn()
	if err != nil {
		return PeerCreds{}, fmt.Errorf("raw conn: %w", err)
	}
	var ucred *unix.Ucred
	var ctlErr error
	if err := raw.Control(func(fd uintptr) {
		ucred, ctlErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return PeerCreds{}, fmt.Errorf("control: %w", err)
	}
	if ctlErr != nil {
		return PeerCreds{}, fmt.Errorf("getsockopt SO_PEERCRED: %w", ctlErr)
	}
	return PeerCreds{Uid: uint32(ucred.Uid), Pid: uint32(ucred.Pid)}, nil
}
