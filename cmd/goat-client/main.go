// Package main is the goat-client desktop GUI binary.
//
// One binary serves two roles:
//
//   - Default (no flags): runs as the systray parent. Holds the tray icon,
//     polls goat-clientd for status, exposes Connect/Disconnect plus an
//     "Open window..." menu item.
//   - --window: runs as the Fyne window child, spawned by the parent on
//     demand. Carries the bundle-import dialog, status pane, and
//     diagnostics tab.
//
// The split is the netbird upstream pattern and exists because
// fyne.io/systray's NSStatusItem and Fyne's NSApplication both want the
// macOS main thread; running them in separate processes is the safe path.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/dlf-dds/goat-client/internal/ipc"
	"github.com/dlf-dds/goat-client/internal/ui"
)

func main() {
	var (
		windowMode = flag.Bool("window", false, "run as the Fyne window child process (otherwise: run the systray)")
		daemonAddr = flag.String("daemon-addr", ipc.DefaultAddr(), "goat-clientd IPC address")
	)
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	if *windowMode {
		log.SetPrefix("goat-client[window] ")
		if err := ui.RunWindow(*daemonAddr); err != nil {
			fmt.Fprintf(os.Stderr, "window: %v\n", err)
			os.Exit(1)
		}
		return
	}

	log.SetPrefix("goat-client[tray] ")
	if err := ui.RunTray(*daemonAddr); err != nil {
		fmt.Fprintf(os.Stderr, "tray: %v\n", err)
		os.Exit(1)
	}
}
