//go:build darwin && cgo

package ipc

// #include <unistd.h>
// #include <sys/types.h>
import "C"
import "fmt"

// getpeereid wraps the BSD libc call getpeereid(2). Returns euid, egid.
// CGO is required to call libc directly. Phase 1 desktop builds run with
// CGO_ENABLED=0 (per Track E reproducibility flags) — see the no-CGO
// fallback in peercreds_darwin_nocgo.go.
func getpeereid(fd int) (uint32, uint32, error) {
	var euid C.uid_t
	var egid C.gid_t
	if rc, err := C.getpeereid(C.int(fd), &euid, &egid); rc != 0 {
		return 0, 0, fmt.Errorf("getpeereid: %w", err)
	}
	return uint32(euid), uint32(egid), nil
}
