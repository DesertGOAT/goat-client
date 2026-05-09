//go:build android

package goatclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Connection states surfaced by GetTunnelStatus(). Constants are exported
// so the Kotlin shell doesn't have to hard-code string literals.
const (
	StateUnconfigured = "unconfigured" // no bundle imported yet
	StateImported     = "imported"     // bundle imported, tunnel not started
	StateConnecting   = "connecting"   // Run() in progress, handshake pending
	StateConnected    = "connected"    // wg-cp0 handshake complete
	StateDisconnected = "disconnected" // tunnel stopped
	StateError        = "error"        // see Reason field
)

// Client is the gomobile-bound entry point. The Kotlin shell holds one
// instance per app process; lifecycle is bound to GoatVpnService.
//
// Reshaped from netbird client/android/Client: Login / IsLoginRequired
// / Networks / PeersList / route management stripped (single-peer
// wg-cp0 has none of those concepts). Added: ImportBundle (file picker
// or QR scan upload path) + GetTunnelStatus (UI poll path).
type Client struct {
	deviceName string
	uiVersion  string

	tunAdapter            TunAdapter
	iFaceDiscover         IFaceDiscover
	networkChangeListener NetworkChangeListener

	mu        sync.RWMutex
	files     PlatformFiles
	state     string
	reason    string // populated when state == StateError
	since     time.Time
	bundleSum string // hex-encoded SHA-256 of last imported bundle, for UI

	ctxCancel context.CancelFunc
}

// NewClient returns a new Client. Lifecycle:
//
//	c := NewClient(31, "Pixel 8", "0.0.1", vpnSvc, vpnSvc, vpnSvc)
//	c.ImportBundle(bytesFromPicker)              // one-time, or on bundle rotation
//	c.Run(platformFiles, dnsList, listener, envList) // blocks until Stop()
//
// The androidSDKVersion arg is a hint to the engine for OS-specific
// workarounds (e.g. pidfd seccomp policy on API ≤30); see netbird
// client/android/exec.go for the analogue.
func NewClient(androidSDKVersion int, deviceName string, uiVersion string, tunAdapter TunAdapter, iFaceDiscover IFaceDiscover, networkChangeListener NetworkChangeListener) *Client {
	setAndroidProtectSocketFn(tunAdapter.ProtectSocket)
	return &Client{
		deviceName:            deviceName,
		uiVersion:             uiVersion,
		tunAdapter:            tunAdapter,
		iFaceDiscover:         iFaceDiscover,
		networkChangeListener: networkChangeListener,
		state:                 StateUnconfigured,
		since:                 time.Now(),
	}
}

// ImportBundle accepts the raw bytes of an offline-CA-signed CBOR bundle
// (per goat-trunk docs/design/offline-enrollment.md) and persists them to
// the configured ConfigurationFilePath. Bundle parse + Ed25519 verify
// against the pinned offline-CA root happens here once internal/bundle
// (Track A) lands its parser; for now the bytes are persisted as-is so
// the Kotlin shell's UX path (file picker -> apply -> connect) can be
// driven end-to-end against the persisted-bytes invariant.
//
// Returns an error on:
//   - empty input (caller should verify file/QR payload before calling)
//   - filesystem write failure (Android sandbox / disk full)
//   - bundle signature verify failure (once Track A wires it)
func (c *Client) ImportBundle(bundleBytes []byte) error {
	if len(bundleBytes) == 0 {
		return errors.New("import bundle: empty payload")
	}
	c.mu.Lock()
	files := c.files
	c.mu.Unlock()

	if files == nil {
		return errors.New("import bundle: PlatformFiles not yet attached; call Run() first or supply files via Configure()")
	}

	cfgPath := files.ConfigurationFilePath()
	if cfgPath == "" {
		return errors.New("import bundle: PlatformFiles.ConfigurationFilePath() returned empty")
	}

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		return fmt.Errorf("import bundle: mkdir parent: %w", err)
	}
	tmp := cfgPath + ".tmp"
	if err := os.WriteFile(tmp, bundleBytes, 0o600); err != nil {
		return fmt.Errorf("import bundle: write temp: %w", err)
	}
	if err := os.Rename(tmp, cfgPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("import bundle: rename: %w", err)
	}

	// TODO(track-A-internal-bundle): once internal/bundle lands its
	// parser, replace this with: parsed, err := bundle.Parse(bundleBytes);
	// if err != nil { return err }; reject if signature verify fails;
	// extract and surface issued-to / site / expires / peer-pubkey via
	// the GetTunnelStatus() JSON for the Kotlin UI.

	sum := bundleChecksum(bundleBytes)
	c.mu.Lock()
	c.bundleSum = sum
	if c.state == StateUnconfigured || c.state == StateError {
		c.state = StateImported
		c.reason = ""
		c.since = time.Now()
	}
	c.mu.Unlock()
	return nil
}

// Configure attaches PlatformFiles without starting the engine. Useful
// when the Kotlin shell wants to support an Import-without-Connect UX
// (user adds a bundle, leaves the app, returns later to start the
// tunnel). Run() also accepts PlatformFiles and will overwrite.
func (c *Client) Configure(files PlatformFiles) {
	c.mu.Lock()
	c.files = files
	c.mu.Unlock()
}

// Run starts the wg-cp0 outer tunnel and blocks until Stop is called or
// a fatal error occurs. Returns nil on clean Stop, error otherwise.
//
// Shape preserved from netbird client/android/Client.Run minus the
// urlOpener+isAndroidTV args (no SSO flow on goat). Args:
//   - files: per-app filesystem paths
//   - dns:   bundle-supplied DNS resolvers (may be empty)
//   - dnsReadyListener: fires when DNS is in effect
//   - envList: tunable env vars (force-relay et al)
//
// The engine wires net.SetAndroidProtectSocketFn(tunAdapter.ProtectSocket)
// during NewClient, so socket protection is live before this call.
//
// TODO(track-A-internal-tunnel): once internal/tunnel lands the
// single-peer wg-cp0 engine, this method calls into it with the
// imported bundle's peer-pubkey + endpoints. Today it returns a
// "not yet wired" error so the Kotlin shell can light up the UI
// flow against the persisted-bundle invariant without depending on
// Track A's mid-flight state.
func (c *Client) Run(files PlatformFiles, dns *DNSList, dnsReadyListener DnsReadyListener, envList *EnvList) error {
	c.mu.Lock()
	c.files = files
	c.state = StateConnecting
	c.reason = ""
	c.since = time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	c.ctxCancel = cancel
	c.mu.Unlock()

	exportEnv(envList)

	if err := c.assertBundlePresent(); err != nil {
		c.fail(err.Error())
		return err
	}

	// TODO(track-A): hand off to internal/tunnel.Engine{}.Run(ctx, ...)
	// passing tunAdapter, iFaceDiscover, networkChangeListener, dns,
	// dnsReadyListener, files, and the parsed bundle. The engine call
	// is blocking; on return, transition to StateDisconnected and
	// return the error (or nil if ctx was cancelled by Stop).
	err := errors.New("wg-cp0 tunnel engine not yet integrated; depends on Track A internal/tunnel converging")
	c.fail(err.Error())

	// Park here until Stop() so the Kotlin foreground service stays
	// alive; otherwise VpnService would be torn down immediately. Once
	// internal/tunnel lands this <- waits on the engine instead.
	<-ctx.Done()
	c.mu.Lock()
	c.state = StateDisconnected
	c.since = time.Now()
	c.mu.Unlock()
	return err
}

// Stop signals the engine to tear down the tunnel. Safe to call
// multiple times; safe to call before Run.
func (c *Client) Stop() {
	c.mu.Lock()
	cancel := c.ctxCancel
	c.ctxCancel = nil
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// RenewTun is called by Kotlin when the system gives back a fresh
// utun fd (e.g. after VpnService.Builder rebuild on a network change).
// The engine swaps the underlying TUN without dropping the WG session.
//
// TODO(track-A): wire to internal/tunnel.Engine.RenewTun(fd) once it lands.
func (c *Client) RenewTun(fd int) error {
	if fd < 0 {
		return fmt.Errorf("renew tun: invalid fd %d", fd)
	}
	return errors.New("renew tun: engine not yet integrated; depends on Track A internal/tunnel converging")
}

// SetTraceLogLevel / SetInfoLogLevel are stubs that match netbird's
// gomobile API surface so the Kotlin shell can compile against either
// engine during the converge window.
func (c *Client) SetTraceLogLevel() {}
func (c *Client) SetInfoLogLevel()  {}

// GetTunnelStatus returns a JSON-encoded status snapshot for the UI:
//
//	{
//	  "state":      "connected",
//	  "reason":     "",
//	  "since":      "2026-05-09T22:30:00Z",
//	  "bundleSum":  "<sha256-hex>",
//	  "deviceName": "Pixel 8"
//	}
//
// Kotlin polls this on the status pane (cheap; in-memory). For
// per-second handshake / bytes-in/out, Track A will add a separate
// streaming RPC; this method stays for the at-a-glance UI poll.
func (c *Client) GetTunnelStatus() string {
	c.mu.RLock()
	snap := struct {
		State      string `json:"state"`
		Reason     string `json:"reason,omitempty"`
		Since      string `json:"since"`
		BundleSum  string `json:"bundleSum,omitempty"`
		DeviceName string `json:"deviceName,omitempty"`
	}{
		State:      c.state,
		Reason:     c.reason,
		Since:      c.since.UTC().Format(time.RFC3339),
		BundleSum:  c.bundleSum,
		DeviceName: c.deviceName,
	}
	c.mu.RUnlock()
	b, err := json.Marshal(snap)
	if err != nil {
		// json.Marshal of a fixed shape only fails on impossible
		// inputs; emit a defensive payload rather than panic across
		// the gomobile boundary.
		return `{"state":"error","reason":"status marshal failed"}`
	}
	return string(b)
}

func (c *Client) assertBundlePresent() error {
	c.mu.RLock()
	files := c.files
	c.mu.RUnlock()
	if files == nil {
		return errors.New("no PlatformFiles attached")
	}
	cfgPath := files.ConfigurationFilePath()
	if cfgPath == "" {
		return errors.New("ConfigurationFilePath empty")
	}
	st, err := os.Stat(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("no bundle imported; call ImportBundle first")
		}
		return fmt.Errorf("stat bundle: %w", err)
	}
	if st.Size() == 0 {
		return errors.New("imported bundle is empty")
	}
	return nil
}

func (c *Client) fail(reason string) {
	c.mu.Lock()
	c.state = StateError
	c.reason = reason
	c.since = time.Now()
	c.mu.Unlock()
}

func exportEnv(envList *EnvList) {
	if envList == nil {
		return
	}
	for k, v := range envList.snapshot() {
		_ = os.Setenv(k, v)
	}
}
