//go:build darwin

package ipc

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// peerCredsUnix reads LOCAL_PEERPID + the effective uid via getpeereid(2)
// on macOS. macOS does not implement SO_PEERCRED — instead, the SOL_LOCAL
// socket option LOCAL_PEERPID returns the peer pid, and getpeereid(2) (a
// libc wrapper around two SOL_LOCAL options) returns the peer's effective
// uid + gid. We use the unix package's exposed wrappers.
func peerCredsUnix(uc *net.UnixConn) (PeerCreds, error) {
	raw, err := uc.SyscallConn()
	if err != nil {
		return PeerCreds{}, fmt.Errorf("raw conn: %w", err)
	}
	var uid uint32
	var pid int32
	var ctlErr error
	if err := raw.Control(func(fd uintptr) {
		// LOCAL_PEERPID = 0x002 in <sys/un.h>; not exported by x/sys/unix
		// as a named constant, so we use the raw value. SOL_LOCAL = 0.
		const localPeerPid = 0x002
		const solLocal = 0
		var pidVal int
		pidVal, ctlErr = unix.GetsockoptInt(int(fd), solLocal, localPeerPid)
		if ctlErr != nil {
			return
		}
		pid = int32(pidVal)
		var euid uint32
		euid, _, ctlErr = getpeereid(int(fd))
		if ctlErr != nil {
			return
		}
		uid = euid
	}); err != nil {
		return PeerCreds{}, fmt.Errorf("control: %w", err)
	}
	if ctlErr != nil {
		return PeerCreds{}, fmt.Errorf("peer creds lookup: %w", ctlErr)
	}
	return PeerCreds{Uid: uid, Pid: uint32(pid)}, nil
}
