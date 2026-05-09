package ui

import "fyne.io/fyne/v2"

// notifier wraps fyne.App's notification surface so the rest of the UI
// package never imports fyne directly for that concern. Cross-platform
// (Windows toast, macOS NSUserNotification, Linux libnotify) is handled
// by the Fyne app implementation.
type notifier struct {
	app fyne.App
}

func newNotifier(app fyne.App) *notifier {
	return &notifier{app: app}
}

func (n *notifier) Send(title, body string) {
	if n == nil || n.app == nil {
		return
	}
	n.app.SendNotification(fyne.NewNotification(title, body))
}
