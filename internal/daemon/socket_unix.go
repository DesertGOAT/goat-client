//go:build unix

package daemon

import (
	"os"
	"path/filepath"
)

// DefaultSocketPath is the per-user IPC socket path. We keep it in the
// user's runtime/cache dir rather than /var/run because the daemon runs
// as the user (not as root) on the desktop spine — root-mode service
// installs ship later via packaging (Track F) and override this via
// flag.
func DefaultSocketPath() string {
	if rt := os.Getenv("XDG_RUNTIME_DIR"); rt != "" {
		return filepath.Join(rt, "goat-clientd.sock")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".goat-client", "goat-clientd.sock")
	}
	return "/tmp/goat-clientd.sock"
}
