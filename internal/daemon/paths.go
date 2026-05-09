package daemon

import (
	"os"
	"path/filepath"
)

// DefaultBundlePath returns the per-user persisted-bundle path. We keep
// the bundle in $XDG_CONFIG_HOME (or ~/.config on Linux), under
// `Application Support/goat-client/` on macOS, and under `%APPDATA%`
// on Windows. The file is mode 0600 — the daemon treats it as
// secret-equivalent because CPDevicePrivkey is inside.
func DefaultBundlePath() string {
	if dir, err := userConfigDir(); err == nil {
		return filepath.Join(dir, "bundle.cbor")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".goat-client", "bundle.cbor")
	}
	return ".goat-client/bundle.cbor"
}

// DefaultTrustRootsPath returns the path operators drop the offline-CA
// trust file at. Distinct from BundlePath because trust roots are the
// daemon's static config — bundle is per-device.
func DefaultTrustRootsPath() string {
	if dir, err := userConfigDir(); err == nil {
		return filepath.Join(dir, "trust-roots.pem")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".goat-client", "trust-roots.pem")
	}
	return ".goat-client/trust-roots.pem"
}

// userConfigDir is os.UserConfigDir + the goat-client subdirectory. It
// does not create the directory — the caller does that on first write.
func userConfigDir() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "goat-client"), nil
}
