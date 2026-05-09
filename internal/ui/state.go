package ui

import "github.com/dlf-dds/goat-client/internal/ipc"

// stateLabel returns the human-readable string the tray + status pane show
// for a given IPC state.
func stateLabel(s ipc.State) string {
	switch s {
	case ipc.StateConnected:
		return "Connected"
	case ipc.StateConnecting:
		return "Connecting..."
	case ipc.StateError:
		return "Error"
	default:
		return "Disconnected"
	}
}
