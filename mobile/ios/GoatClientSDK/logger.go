//go:build ios

package GoatClientSDK

import (
	"fmt"
	"os"
)

// InitializeLog is gomobile-callable from the Swift NEPacketTunnelProvider.
// Sets up file-backed logging at filePath with the requested level.
//
// Levels: "trace", "debug", "info", "warn", "error". Anything else falls
// back to "info".
//
// Until Track A wires in a structured logger (logrus / slog), this is a
// thin pass-through that just records the request to stderr — sufficient
// for iOS Simulator engineering builds.
func InitializeLog(logLevel string, filePath string) error {
	// Belt-and-suspenders: if filePath is set, make sure the parent dir
	// exists and is writable. Don't fail hard if not — Swift may pass an
	// App Group container that already exists, and we'd rather log than
	// abort.
	if filePath != "" {
		f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "GoatClientSDK: log file %q not openable (%v); falling back to stderr\n", filePath, err)
		} else {
			_ = f.Close()
		}
	}
	fmt.Fprintf(os.Stderr, "GoatClientSDK: log initialized (level=%s, path=%q)\n", logLevel, filePath)
	return nil
}
