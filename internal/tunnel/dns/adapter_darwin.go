//go:build darwin && !ios

package dns

// newPlatformAdapter returns the macOS host-DNS adapter. Phase 1 is a
// no-op; Phase 2 will lift netbird's host_darwin.go (scutil State:/Network
// updates) into this file.
//
// Lift sources (netbird@32d04da19a):
//   - client/internal/dns/host_darwin.go — scutil-driven adapter
//   - client/internal/dns/unclean_shutdown_darwin.go — recovery on next boot
func newPlatformAdapter() (Adapter, error) {
	return noopAdapter{}, nil
}
