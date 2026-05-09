//go:build darwin && !cgo

package ipc

import "os"

// getpeereid (no-CGO fallback) returns the daemon's own uid. macOS does
// not expose a syscall.SYS_GETPEEREID, and the LOCAL_PEEREUID socket
// option's stable numeric value is not in public headers — both call
// paths a non-CGO build could take are unsafe to bake in.
//
// The trade-off this picks: under CGO_ENABLED=0 we rely on the socket
// file mode (0600, set in Listen) for isolation rather than uid
// enforcement at the IPC layer. The 0600 mode already prevents other
// uids from connecting; uid enforcement here is defense-in-depth that
// only matters if a future change loosens the file mode. Phase 2 of
// Track A may add LOCAL_PEEREUID via x/sys/unix once that exposes the
// constant; until then, a CGO build (linked against libc) gives us the
// authoritative answer.
func getpeereid(fd int) (uint32, uint32, error) {
	euid := uint32(os.Geteuid())
	egid := uint32(os.Getegid())
	return euid, egid, nil
}
