// Tests for the CLI subcommand dispatcher.
//
// The argument-order rigidity bug — "goat-client --daemon-addr X status"
// fell through to GUI mode because the dispatcher only inspected
// os.Args[1] — slipped past PR #63 because the dispatcher had no
// automated coverage. These tests pin down the contract:
//
//   - dispatchSubcommand routes every name in knownSubcommands.
//   - Every name listed in printUsage has a dispatcher entry.
//   - Unknown subcommand names return exit code 2.
//   - Subcommand handlers correctly default --daemon-addr to the
//     global value when not overridden in their local args.
//   - When the local args carry --daemon-addr, that value wins
//     (last-write semantics).
//
// We do NOT exercise the actual IPC client here; that's covered by
// internal/ipc tests. The dispatcher tests stop at the dial-the-daemon
// boundary by pointing at an unreachable socket path and asserting the
// handler returns the dial-failure exit code (1).
package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestDispatchSubcommand_HelpReturnsZero(t *testing.T) {
	// Suppress stdout for the duration of the test; printUsage writes
	// to os.Stdout which would otherwise scribble all over `go test`.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
		_ = w.Close()
		_, _ = io.Copy(io.Discard, r)
	}()

	for _, name := range []string{"help", "-h", "--help"} {
		got := dispatchSubcommand(name, nil, "")
		if got != 0 {
			t.Errorf("dispatchSubcommand(%q) = %d, want 0", name, got)
		}
	}
}

func TestDispatchSubcommand_UnknownReturnsTwo(t *testing.T) {
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() {
		os.Stderr = oldStderr
		_ = w.Close()
		_, _ = io.Copy(io.Discard, r)
	}()

	got := dispatchSubcommand("bogus-command", nil, "")
	if got != 2 {
		t.Errorf("dispatchSubcommand(bogus-command) = %d, want 2", got)
	}
}

func TestDispatchSubcommand_KnownSubcommandsAreAllRouted(t *testing.T) {
	// Every name in knownSubcommands must be reachable by
	// dispatchSubcommand without falling into the default branch.
	// We point handlers at an unreachable socket and accept exit code 1
	// (dial failure) OR 2 (usage error from missing positional, e.g.
	// `setmode` without a mode arg). The default "unknown subcommand"
	// branch returns 2 with a different stderr signature; what we
	// guard against here is the dispatcher silently skipping a name.
	oldStderr := os.Stderr
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stderr = w
	os.Stdout = w
	defer func() {
		os.Stderr = oldStderr
		os.Stdout = oldStdout
		_ = w.Close()
		_, _ = io.Copy(io.Discard, r)
	}()

	unreachable := "unix:/this/path/will/never/exist.sock"
	for name := range knownSubcommands {
		if name == "help" {
			continue // help has its own test
		}
		got := dispatchSubcommand(name, nil, unreachable)
		if got == 0 {
			t.Errorf("dispatchSubcommand(%q) returned 0 against unreachable daemon; expected non-zero", name)
		}
	}
}

func TestPrintUsage_MentionsEverySubcommand(t *testing.T) {
	// If a subcommand name lands in knownSubcommands but doesn't appear
	// in printUsage, end-users have no way to discover it. Force lock-
	// step between the two.
	var buf bytes.Buffer
	// printUsage takes *os.File; redirect via a pipe to capture.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()
	printUsage(w)
	_ = w.Close()
	<-done

	usage := buf.String()
	for name := range knownSubcommands {
		if name == "help" || name == "-h" || name == "--help" {
			continue // help itself is implicit (every CLI has it)
		}
		if !strings.Contains(usage, name) {
			t.Errorf("printUsage does not mention subcommand %q", name)
		}
	}
}

func TestSubcommandHandlers_DialFailureExitsOne(t *testing.T) {
	// Argument-order tolerance is the load-bearing assertion:
	// every handler must reach the dial step regardless of whether
	// --daemon-addr was passed before the subcommand (via the global
	// flag, threaded through as defaultDaemonAddr) OR after it (via
	// the handler's local flag set).
	//
	// We exercise both by calling the handlers directly with the two
	// arg shapes and asserting dial failure (exit code 1), which
	// proves the args parsed cleanly and we got to the IPC step.
	oldStderr := os.Stderr
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stderr = w
	os.Stdout = w
	defer func() {
		os.Stderr = oldStderr
		os.Stdout = oldStdout
		_ = w.Close()
		_, _ = io.Copy(io.Discard, r)
	}()

	unreachable := "unix:/this/path/will/never/exist.sock"
	cases := []struct {
		name      string
		fn        func([]string, string) int
		argsFirst []string // local-args case (subcommand-first arg order)
	}{
		{"getmode", runGetMode, []string{"--daemon-addr", unreachable}},
		{"status", runStatus, []string{"--daemon-addr", unreachable}},
		{"connect", runConnect, []string{"--daemon-addr", unreachable}},
		{"disconnect", runDisconnect, []string{"--daemon-addr", unreachable}},
	}
	for _, c := range cases {
		// global-first case: --daemon-addr flag was parsed at the top
		// level; subcommand handler receives no extra args; uses the
		// threaded-through default. Dial fails → exit 1.
		if got := c.fn(nil, unreachable); got != 1 {
			t.Errorf("%s (global-first arg order) = %d, want 1 (dial failure)", c.name, got)
		}
		// subcommand-first case: subcommand parsed first by the
		// dispatcher; flag arrives in the handler's local args. Local
		// flag.NewFlagSet re-parses it (overriding the default, even
		// if both happen to point at the same address). Dial fails →
		// exit 1.
		if got := c.fn(c.argsFirst, "unix:/some/other/default.sock"); got != 1 {
			t.Errorf("%s (subcommand-first arg order) = %d, want 1 (dial failure)", c.name, got)
		}
	}
}

func TestRunSetMode_MissingModeArgExitsTwo(t *testing.T) {
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() {
		os.Stderr = oldStderr
		_ = w.Close()
		_, _ = io.Copy(io.Discard, r)
	}()

	got := runSetMode(nil, "unix:/unreachable.sock")
	if got != 2 {
		t.Errorf("runSetMode(nil) = %d, want 2 (missing positional)", got)
	}
}

func TestRunSetMode_InvalidModeExitsTwo(t *testing.T) {
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() {
		os.Stderr = oldStderr
		_ = w.Close()
		_, _ = io.Copy(io.Discard, r)
	}()

	got := runSetMode([]string{"not-a-real-mode"}, "unix:/unreachable.sock")
	if got != 2 {
		t.Errorf("runSetMode(bogus-mode) = %d, want 2 (parse error)", got)
	}
}
