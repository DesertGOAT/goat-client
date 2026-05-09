package ui

import (
	"context"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"fyne.io/systray"

	"github.com/dlf-dds/goat-client/internal/ipc"
)

// trayApp is the parent-process view: just a systray. The Fyne window
// runs as a separate child process spawned on demand. This mirrors the
// netbird upstream pattern and avoids the macOS main-thread conflict
// between fyne.io/systray's NSStatusItem and Fyne's NSApplication.
type trayApp struct {
	addr   string
	client ipc.Client

	mu        sync.Mutex
	lastState ipc.State

	mStatus     *systray.MenuItem
	mConnect    *systray.MenuItem
	mDisconnect *systray.MenuItem
	mOpen       *systray.MenuItem
	mImport     *systray.MenuItem
	mQuit       *systray.MenuItem

	pollCancel context.CancelFunc
}

// RunTray runs the systray loop on the calling goroutine (which MUST be
// the main goroutine on macOS). Returns when the user picks Quit or the
// tray library exits.
func RunTray(addr string) error {
	client, err := ipc.NewClient(addr)
	if err != nil {
		return err
	}
	t := &trayApp{addr: addr, client: client, lastState: ipc.StateDisconnected}
	systray.Run(t.onReady, t.onExit)
	return nil
}

func (t *trayApp) onReady() {
	systray.SetIcon(iconForState(ipc.StateDisconnected))
	systray.SetTitle("")
	systray.SetTooltip("goat-client")

	t.mStatus = systray.AddMenuItem("Status: Disconnected", "Current tunnel state")
	t.mStatus.Disable()
	systray.AddSeparator()
	t.mConnect = systray.AddMenuItem("Connect", "Bring the wg-cp0 tunnel up")
	t.mDisconnect = systray.AddMenuItem("Disconnect", "Bring the wg-cp0 tunnel down")
	systray.AddSeparator()
	t.mOpen = systray.AddMenuItem("Open window...", "Open the goat-client window")
	t.mImport = systray.AddMenuItem("Import bundle...", "Open the bundle import dialog")
	systray.AddSeparator()
	t.mQuit = systray.AddMenuItem("Quit", "Quit goat-client")

	go t.handleClicks()
	t.startPolling()
}

func (t *trayApp) onExit() {
	t.stopPolling()
	if t.client != nil {
		_ = t.client.Close()
	}
}

func (t *trayApp) handleClicks() {
	for {
		select {
		case <-t.mConnect.ClickedCh:
			t.connect()
		case <-t.mDisconnect.ClickedCh:
			t.disconnect()
		case <-t.mOpen.ClickedCh:
			t.spawnWindow()
		case <-t.mImport.ClickedCh:
			// Same child binary; child opens to the Bundle tab.
			t.spawnWindow()
		case <-t.mQuit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func (t *trayApp) connect() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := t.client.Connect(ctx); err != nil {
		log.Printf("tray: connect: %v", err)
	}
	t.refresh(ctx)
}

func (t *trayApp) disconnect() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := t.client.Disconnect(ctx); err != nil {
		log.Printf("tray: disconnect: %v", err)
	}
	t.refresh(ctx)
}

// spawnWindow execs a fresh copy of this binary with --window so the Fyne
// window runs in its own process. Stdout/stderr are inherited so log
// output is visible if the user launched goat-client from a terminal.
func (t *trayApp) spawnWindow() {
	exe, err := os.Executable()
	if err != nil {
		log.Printf("tray: locate executable: %v", err)
		return
	}
	cmd := exec.Command(exe, "--window", "--daemon-addr="+t.addr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Printf("tray: spawn window: %v", err)
		return
	}
	go func() { _ = cmd.Wait() }()
}

func (t *trayApp) startPolling() {
	ctx, cancel := context.WithCancel(context.Background())
	t.pollCancel = cancel
	go func() {
		ticker := time.NewTicker(1500 * time.Millisecond)
		defer ticker.Stop()
		t.refresh(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				t.refresh(ctx)
			}
		}
	}()
}

func (t *trayApp) stopPolling() {
	if t.pollCancel != nil {
		t.pollCancel()
		t.pollCancel = nil
	}
}

func (t *trayApp) refresh(ctx context.Context) {
	st, err := t.client.GetStatus(ctx)
	if err != nil {
		return
	}
	t.mu.Lock()
	changed := st.State != t.lastState
	t.lastState = st.State
	t.mu.Unlock()

	if changed {
		systray.SetIcon(iconForState(st.State))
	}
	if t.mStatus != nil {
		t.mStatus.SetTitle("Status: " + stateLabel(st.State))
	}
	if t.mConnect != nil && t.mDisconnect != nil {
		switch st.State {
		case ipc.StateConnected, ipc.StateConnecting:
			t.mConnect.Disable()
			t.mDisconnect.Enable()
		default:
			t.mConnect.Enable()
			t.mDisconnect.Disable()
		}
	}
}
