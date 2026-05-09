//go:build ios

package GoatClientSDK

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
)

// ErrTrackANotYetWired is returned by tunnel-affecting methods until Track A
// (internal/tunnel + internal/bundle) lands. The Swift shell can render this
// gracefully ("daemon backend not yet integrated") so engineering can iterate
// on the UI + xcframework build pipeline before the Go tunnel core is ready.
var ErrTrackANotYetWired = errors.New("goat-client iOS SDK: tunnel backend not yet wired (Track A pending)")

// ErrNoBundleImported is returned by Run() if ImportBundle hasn't yet been
// called successfully. The bundle carries the wg-cp0 peer pubkey + endpoints
// + assigned tunnel address; without it the daemon has no config to apply.
var ErrNoBundleImported = errors.New("goat-client iOS SDK: no bundle imported (call ImportBundle first)")

// TunnelState mirrors the (very narrow) tunnel status surface exposed to
// Swift. Single peer (wg-cp0), so no peer list — just the one connection.
const (
	StateDisconnected = "disconnected"
	StateConnecting   = "connecting"
	StateConnected    = "connected"
	StateError        = "error"
)

// Client is the gomobile-bound facade the Swift NEPacketTunnelProvider
// instantiates. Lifecycle: NewClient -> ImportBundle (once per fresh
// install or rotation) -> Run (blocking; called from the NE extension's
// startTunnel) -> Stop (called from stopTunnel).
//
// Heavily reshaped from netbird's NetBirdSDK.Client — login/OAuth machinery
// is gone (single-tunnel offline-bundle onboarding model; see
// docs/design/goat-client.md §"Bundle import vs OAuth login"), and per-peer
// mesh status is collapsed to a single peer state.
type Client struct {
	cfgDir                string // App Group container path (writable by both app + NE extension)
	stateFile             string // persistent tunnel state JSON path within cfgDir
	deviceName            string
	osName                string
	osVersion             string
	networkChangeListener NetworkChangeListener
	dnsManager            DnsManager

	mu             sync.Mutex
	ctxCancel      context.CancelFunc
	bundleImported atomic.Bool
	stateAtomic    atomic.Value // string — current StateXxx
}

// NewClient is gomobile-callable. cfgDir is typically the App Group container
// shared between the main app (which calls ImportBundle) and the NE extension
// (which calls Run). stateFile is a JSON file Track A's tunnel will read/write
// for last-handshake / bytes counters.
//
// networkChangeListener and dnsManager are Swift-implemented (see listeners.go).
// Both may be nil for unit-style smoke tests; production callers should wire
// real implementations in the NEPacketTunnelProvider.
func NewClient(
	cfgDir string,
	stateFile string,
	deviceName string,
	osVersion string,
	osName string,
	networkChangeListener NetworkChangeListener,
	dnsManager DnsManager,
) *Client {
	c := &Client{
		cfgDir:                cfgDir,
		stateFile:             stateFile,
		deviceName:            deviceName,
		osName:                osName,
		osVersion:             osVersion,
		networkChangeListener: networkChangeListener,
		dnsManager:            dnsManager,
	}
	c.stateAtomic.Store(StateDisconnected)
	return c
}

// ImportBundle takes the raw bytes of an offline-CA-signed CBOR bundle (see
// docs/design/offline-enrollment.md and goat-trunk's
// ops/enrollment/cmd/bundle-extract for the format) and persists the parsed
// config under cfgDir.
//
// gomobile cannot bind a Go []byte parameter directly — it gets bridged to
// Swift Data / NSData. Swift callers pass the raw bundle file contents read
// via UIDocumentPicker (file-picker import) or AVFoundation (QR scan + base64
// decode).
//
// Wiring: once internal/bundle (Track A) lands, this dispatches to
// bundle.ParseAndVerify(bundleBytes) and persists. Until then it validates
// shape (non-empty + minimum size) and stashes the raw bytes for Run().
func (c *Client) ImportBundle(bundleBytes []byte) error {
	if len(bundleBytes) == 0 {
		return fmt.Errorf("empty bundle")
	}
	if len(bundleBytes) < 64 {
		// CBOR header + Ed25519 signature (64 bytes) alone is well over 64
		// bytes; anything smaller is definitely garbage.
		return fmt.Errorf("bundle too small (%d bytes); not a valid CBOR-signed bundle", len(bundleBytes))
	}
	if c.cfgDir == "" {
		return fmt.Errorf("client cfgDir not configured (NewClient called with empty cfgDir)")
	}

	// Persist for Run() to pick up. Track A's bundle.ParseAndVerify will
	// replace this raw write with verified-then-persisted-config; for now we
	// just stash the bytes so the round-trip is exercised end-to-end.
	bundlePath := c.cfgDir + "/bundle.cbor"
	if err := os.MkdirAll(c.cfgDir, 0o700); err != nil {
		return fmt.Errorf("mkdir cfgDir: %w", err)
	}
	if err := os.WriteFile(bundlePath, bundleBytes, 0o600); err != nil {
		return fmt.Errorf("write bundle: %w", err)
	}
	c.bundleImported.Store(true)
	return nil
}

// Run starts the tunnel goroutine and blocks until Stop is called or the
// underlying tunnel exits. Called from the Swift NEPacketTunnelProvider's
// startTunnel(options:completionHandler:) — typically on a background
// dispatch queue.
//
// fd: the utun file descriptor the NEPacketTunnelProvider's packetFlow exposes
// (Swift extracts it via the well-known socket dance).
//
// interfaceName: the wg interface name (e.g. "utun5"); informational, the FD
// is what actually gets driven.
//
// envList: optional env-var bag pre-applied to os.Setenv before tunnel start.
// May be nil.
//
// Currently a stub that records connecting->error transition and returns
// ErrTrackANotYetWired. Once Track A lands tunnel.RunOniOS, the wiring drops
// in here.
func (c *Client) Run(fd int32, interfaceName string, envList *EnvList) error {
	if !c.bundleImported.Load() {
		// Permit Run() if a bundle was previously persisted to cfgDir
		// (e.g. main app imported, then process restarted, NE extension is
		// running fresh).
		if _, err := os.Stat(c.cfgDir + "/bundle.cbor"); err != nil {
			c.stateAtomic.Store(StateError)
			return ErrNoBundleImported
		}
	}

	c.mu.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	c.ctxCancel = cancel
	c.mu.Unlock()
	defer cancel()

	applyEnv(envList)
	c.stateAtomic.Store(StateConnecting)

	// TODO(track-a): replace with tunnel.RunOniOS(ctx, fd, interfaceName,
	// c.cfgDir, c.networkChangeListener, c.dnsManager). The signatures here
	// are designed to drop in cleanly: ctx for cancellation, fd for the utun
	// bridge (per netbird device_ios.go), the listener interfaces for
	// path-monitor + NEDNSSettings.
	_ = ctx
	_ = fd
	_ = interfaceName
	c.stateAtomic.Store(StateError)
	return ErrTrackANotYetWired
}

// Stop signals the running tunnel to wind down. Idempotent — safe to call
// multiple times or before Run.
func (c *Client) Stop() {
	c.mu.Lock()
	cancel := c.ctxCancel
	c.ctxCancel = nil
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	c.stateAtomic.Store(StateDisconnected)
}

// GetTunnelStatus returns one of StateDisconnected / StateConnecting /
// StateConnected / StateError. The Swift main-app polls this for the
// status pane; the NEPacketTunnelProvider also reads it after Run returns.
func (c *Client) GetTunnelStatus() string {
	v, _ := c.stateAtomic.Load().(string)
	if v == "" {
		return StateDisconnected
	}
	return v
}

// SetCustomLogger lets Swift attach an os_log-backed logger after NewClient.
// Optional. Currently a no-op until logger.go grows a real sink — recorded
// here so the gomobile binding API is stable.
func (c *Client) SetCustomLogger(_ CustomLogger) {
	// reserved
}

func applyEnv(list *EnvList) {
	for k, v := range list.allItems() {
		_ = os.Setenv(k, v)
	}
}
