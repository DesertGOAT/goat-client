package ui

import (
	"fyne.io/fyne/v2/app"

	"github.com/dlf-dds/goat-client/internal/ipc"
)

// RunWindow starts the Fyne main window. Blocks until the user closes the
// window. MUST be called on the main goroutine on macOS — this is the
// child-process role; the parent (RunTray) handles the systray.
func RunWindow(addr string) error {
	client, err := ipc.NewClient(addr)
	if err != nil {
		return err
	}
	defer client.Close()

	a := app.NewWithID("io.dlf-dds.goat-client")
	mw := newMainWindow(a, client)
	mw.Show()
	return nil
}
