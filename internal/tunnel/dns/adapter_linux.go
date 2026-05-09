//go:build linux && !android

package dns

// newPlatformAdapter returns the Linux host-DNS adapter. Phase 1 is a
// no-op; Phase 2 will lift netbird's host_unix.go (systemd-resolved
// dbus integration) + the file-based resolv.conf fallback in
// file_unix.go / file_repair_unix.go, structured around the Adapter
// interface in dns.go.
//
// Lift sources (netbird@32d04da19a):
//   - client/internal/dns/host_unix.go — Adapter dispatch (systemd vs file)
//   - client/internal/dns/dbus_unix.go — systemd-resolved D-Bus
//   - client/internal/dns/network_manager_unix.go — NM fallback
//   - client/internal/dns/resolvconf_unix.go — resolvconf(8) fallback
//   - client/internal/dns/file_unix.go — last-resort /etc/resolv.conf rewrite
//   - client/internal/dns/file_repair_unix.go — restore on shutdown
//   - client/internal/dns/file_parser_unix.go — resolv.conf parser
func newPlatformAdapter() (Adapter, error) {
	return noopAdapter{}, nil
}
