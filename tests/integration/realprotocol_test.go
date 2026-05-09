//go:build realprotocol

package integration

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/dlf-dds/goat-client/internal/ipc"
)

// TestRealProtocol_BundleImportAndHandshake brings up the real wg-cp0
// outer tunnel against an endpoint in the goat sandbox lab, using a
// CBOR bundle minted by the offline-CA workstation and the matching
// trust-root PEM. Sibling to the in-process integration tests; same
// harness shape, real bundle + real driver.
//
// Required env:
//
//   - GOAT_LAB_BUNDLE_PATH        path to a CBOR-encoded offline-CA bundle
//                                 valid for a wg-cp0 endpoint in the lab.
//   - GOAT_LAB_TRUST_ROOTS_PATH   PEM file with the offline-CA Ed25519
//                                 pubkey that signed the bundle.
//
// Optional env:
//
//   - GOAT_LAB_TIMEOUT_SEC        handshake-deadline override; default 30.
//
// CI: nightly job runs this with the env set. PR jobs do NOT run it
// (skipped when env is empty).
func TestRealProtocol_BundleImportAndHandshake(t *testing.T) {
	bundlePath := os.Getenv("GOAT_LAB_BUNDLE_PATH")
	trustPath := os.Getenv("GOAT_LAB_TRUST_ROOTS_PATH")
	if bundlePath == "" || trustPath == "" {
		t.Skip("GOAT_LAB_BUNDLE_PATH / GOAT_LAB_TRUST_ROOTS_PATH not set; sandbox lab smoke skipped")
	}

	bundleBytes, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatalf("read lab bundle: %v", err)
	}
	pemBytes, err := os.ReadFile(trustPath)
	if err != nil {
		t.Fatalf("read lab trust roots: %v", err)
	}

	timeoutSec := 30
	if v := os.Getenv("GOAT_LAB_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeoutSec = n
		}
	}

	// Build a fixture rooted in the lab artifacts (newFixture mints its
	// own keypair, so we hand-construct one pointing at operator-
	// supplied files).
	dir := shortTempDir(t)
	labTrustPath := filepath.Join(dir, "trust-roots.pem")
	if err := os.WriteFile(labTrustPath, pemBytes, 0o600); err != nil {
		t.Fatalf("write trust roots: %v", err)
	}
	fix := &fixture{
		BundleBytes:     bundleBytes,
		TrustRootsPEM:   pemBytes,
		TrustRootsPath:  labTrustPath,
		SocketPath:      filepath.Join(dir, "ipc.sock"),
		BundleStatePath: filepath.Join(dir, "bundle.cbor"),
	}

	d := startDaemon(t, fix)

	info, err := d.client.ImportBundle(testCtx(), fix.BundleBytes)
	if err != nil {
		t.Fatalf("importBundle (real lab bundle): %v", err)
	}
	t.Logf("imported bundle: issued_to=%s site=%s expires=%s",
		info.IssuedTo, info.Site, info.NotAfter.Format(time.RFC3339))

	if err := d.client.Connect(testCtx()); err != nil {
		t.Fatalf("connect to lab wg-cp0: %v", err)
	}

	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	for time.Now().Before(deadline) {
		st, err := d.client.GetStatus(testCtx())
		if err != nil {
			t.Fatalf("getStatus during handshake wait: %v", err)
		}
		if st.State == ipc.StateConnected && !st.LastHandshake.IsZero() {
			t.Logf("handshake confirmed: in=%d out=%d last_handshake=%s",
				st.BytesIn, st.BytesOut, st.LastHandshake.Format(time.RFC3339))
			if err := d.client.Disconnect(testCtx()); err != nil {
				t.Errorf("disconnect (post-success): %v", err)
			}
			return
		}
		time.Sleep(500 * time.Millisecond)
	}

	finalStatus, _ := d.client.GetStatus(testCtx())
	_ = d.client.Disconnect(testCtx())
	t.Fatalf("wg-cp0 handshake did not complete within %ds; final status=%+v", timeoutSec, finalStatus)
}
