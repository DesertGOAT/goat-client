//go:build integration

package integration

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dlf-dds/goat-client/internal/ipc"
)

// TestImportBundle_HappyPath: daemon accepts a freshly-signed CBOR bundle
// whose signature verifies against the configured trust root, returns
// metadata that round-trips the device ID + site, and post-import
// GetStatus reflects BundleImported=true.
func TestImportBundle_HappyPath(t *testing.T) {
	fix := newFixture(t)
	d := startDaemon(t, fix)

	// Pre-import: BundleImported=false, state is disconnected (the wire
	// state is "no-bundle" but mapWireState collapses that to the
	// GUI-facing StateDisconnected; GUI distinguishes via
	// BundleImported).
	statusBefore, err := d.client.GetStatus(testCtx())
	if err != nil {
		t.Fatalf("getStatus pre-import: %v", err)
	}
	if statusBefore.BundleImported {
		t.Errorf("pre-import BundleImported=true, want false")
	}
	if statusBefore.State != ipc.StateDisconnected {
		t.Errorf("pre-import state = %s, want %s", statusBefore.State, ipc.StateDisconnected)
	}

	got, err := d.client.ImportBundle(testCtx(), fix.BundleBytes)
	if err != nil {
		t.Fatalf("importBundle: %v", err)
	}
	if got == nil {
		t.Fatalf("importBundle returned nil bundle info")
	}
	if got.IssuedTo != fix.DeviceID {
		t.Errorf("issued_to = %q, want %q", got.IssuedTo, fix.DeviceID)
	}
	if got.Site != fix.Site {
		t.Errorf("site = %q, want %q", got.Site, fix.Site)
	}

	statusAfter, err := d.client.GetStatus(testCtx())
	if err != nil {
		t.Fatalf("getStatus post-import: %v", err)
	}
	if !statusAfter.BundleImported {
		t.Errorf("post-import BundleImported=false, want true")
	}
	if statusAfter.Bundle == nil {
		t.Fatalf("post-import status.bundle is nil")
	}
	if statusAfter.Bundle.Site != fix.Site {
		t.Errorf("post-import status.bundle.site = %q, want %q", statusAfter.Bundle.Site, fix.Site)
	}
}

// TestImportBundle_EmptyRejected: the daemon refuses an empty bundle.
func TestImportBundle_EmptyRejected(t *testing.T) {
	fix := newFixture(t)
	d := startDaemon(t, fix)

	if _, err := d.client.ImportBundle(testCtx(), nil); err == nil {
		t.Fatal("expected error for empty bundle, got nil")
	}
}

// TestImportBundle_BadSignatureRejected: a bundle signed by an unrelated
// Ed25519 key is rejected (TrustRoots.VerifyBundle returns
// ErrUntrustedBundle).
func TestImportBundle_BadSignatureRejected(t *testing.T) {
	fix := newFixture(t)
	d := startDaemon(t, fix)

	// Mint a second fixture's bundle (signed by a different keypair) but
	// keep the first fixture's daemon (which trusts only the first
	// fixture's pubkey). The daemon must reject the second bundle.
	other := newFixture(t)
	if _, err := d.client.ImportBundle(testCtx(), other.BundleBytes); err == nil {
		t.Fatal("expected error for foreign-signed bundle, got nil")
	} else if !strings.Contains(err.Error(), "signature") && !strings.Contains(err.Error(), "trust") && !strings.Contains(err.Error(), "verify") {
		t.Errorf("error message %q does not mention signature/trust/verify; daemon may not be running TrustRoots check", err.Error())
	}
}

// TestConnect_BeforeImport_Rejected: the daemon refuses Connect when no
// bundle is loaded. Asserts the daemon's "no bundle loaded — call
// importBundle first" error path.
func TestConnect_BeforeImport_Rejected(t *testing.T) {
	fix := newFixture(t)
	d := startDaemon(t, fix)

	err := d.client.Connect(testCtx())
	if err == nil {
		t.Fatal("expected error connecting before import, got nil")
	}
	if !strings.Contains(err.Error(), "bundle") {
		t.Errorf("error %q should mention bundle", err.Error())
	}
}

// TestGetDiagnostics_Populated: getDiagnostics returns a non-nil log
// tail + a sane uptime / structure shape. Exercises the diagnostics
// surface end-to-end through the real binary.
func TestGetDiagnostics_Populated(t *testing.T) {
	fix := newFixture(t)
	d := startDaemon(t, fix)

	if _, err := d.client.ImportBundle(testCtx(), fix.BundleBytes); err != nil {
		t.Fatalf("importBundle: %v", err)
	}
	diag, err := d.client.GetDiagnostics(testCtx())
	if err != nil {
		t.Fatalf("getDiagnostics: %v", err)
	}
	if diag == nil {
		t.Fatal("getDiagnostics returned nil")
	}
	// Daemon writes a log line on bundle import; LogTail should be
	// non-empty after a successful import.
	if len(diag.LogTail) == 0 {
		t.Errorf("expected log_tail non-empty after import; got empty")
	}
}

// TestImportBundle_PersistedAcrossRestart: the daemon writes the bundle
// to BundlePath atomically; a second daemon spawn against the same
// state path picks it up via LoadPersistedBundle and reports
// BundleImported=true without any importBundle call.
func TestImportBundle_PersistedAcrossRestart(t *testing.T) {
	fix := newFixture(t)
	d1 := startDaemon(t, fix)
	if _, err := d1.client.ImportBundle(testCtx(), fix.BundleBytes); err != nil {
		t.Fatalf("importBundle: %v", err)
	}
	// Drop the first daemon's IPC connection cleanly; t.Cleanup will
	// terminate the process. We can't `kill + restart` in the same
	// fixture (socket path collision), so spin a fresh fixture using
	// the SAME bundle-state and trust-roots files.
	_ = d1.client.Close()

	// Spawn a second daemon against the same persisted bundle path and
	// trust roots, with a fresh socket path.
	fix2 := *fix
	fix2.SocketPath = filepath.Join(filepath.Dir(fix.SocketPath), "ipc2.sock")
	// The first daemon process is still alive on its socket; we won't
	// touch it. The second daemon binds a different socket and reads
	// the persisted bundle from BundleStatePath via LoadPersistedBundle.
	d2 := startDaemon(t, &fix2)

	st, err := d2.client.GetStatus(testCtx())
	if err != nil {
		t.Fatalf("getStatus on restarted daemon: %v", err)
	}
	if !st.BundleImported {
		t.Errorf("expected BundleImported=true on restarted daemon (persisted bundle should auto-load); got false")
	}
	if st.Bundle == nil || st.Bundle.Site != fix.Site {
		t.Errorf("expected restarted daemon to expose persisted bundle metadata; got %+v", st.Bundle)
	}
}
