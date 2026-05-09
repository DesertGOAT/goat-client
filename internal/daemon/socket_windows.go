//go:build windows

package daemon

// DefaultSocketPath returns the named-pipe name for the IPC server.
// Per-user pipes don't have a uid concept on Windows; the SDDL set in
// internal/ipc/transport_windows.go restricts access to the owning user.
func DefaultSocketPath() string {
	return `\\.\pipe\goat-clientd`
}
