//go:build windows

package dns

// newPlatformAdapter returns the Windows host-DNS adapter. Phase 1 is a
// no-op; Phase 2 will lift netbird's host_windows.go (NRPT — Name
// Resolution Policy Table — for split-DNS) into this file.
//
// Lift sources (netbird@32d04da19a):
//   - client/internal/dns/host_windows.go — NRPT registry-driven adapter
func newPlatformAdapter() (Adapter, error) {
	return noopAdapter{}, nil
}
