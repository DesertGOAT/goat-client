//go:build android

package goatclient

import (
	"fmt"
	"net/netip"
	"sync"
)

// TunAdapter is the bridge between the Kotlin VpnService and the Go
// WireGuard engine. The Kotlin shell implements this interface; the Go
// engine calls into it to acquire the utun fd from the system VpnService
// and to mark outbound sockets as not-routed-through-the-tun (so the
// outer wg-cp0 socket itself doesn't loop back through itself).
//
// Shape preserved from netbird client/iface/device/adapter.go to stay
// drop-in-compatible with internal/tunnel/device/* (which Track A is
// forking from netbird client/iface/device/). Goat is single-peer
// wg-cp0 so `routes` is in practice a single 0.0.0.0/0 (or a tighter
// CIDR if the bundle constrains it) and dns/searchDomains may be empty.
type TunAdapter interface {
	// ConfigureInterface asks the Kotlin VpnService to build the VPN
	// tunnel via android.net.VpnService.Builder, returning the file
	// descriptor of the resulting tun device. mtu is in bytes.
	// routes/searchDomains are semicolon-separated.
	ConfigureInterface(address string, mtu int, dns string, searchDomains string, routes string) (int, error)
	// UpdateAddr re-anchors the tunnel address without a full rebuild.
	// Currently a no-op on Android (VpnService.Builder does not support
	// in-place reconfigure); the engine handles re-establishment by
	// rotating the tunnel.
	UpdateAddr(address string) error
	// ProtectSocket marks the given native socket fd so that traffic on
	// it is sent over the underlying network instead of being routed
	// back into the VPN. Returns true on success.
	ProtectSocket(fd int32) bool
}

// IFaceDiscover surfaces local interface enumeration to ICE / connectivity
// probes. Goat's wg-cp0 mode does not perform ICE-style negotiation, but
// the engine still consults this to pick a binding for the outer UDP
// socket. The Kotlin shell can return NetworkInterface.getNetworkInterfaces
// flattened to "name=ip" strings.
type IFaceDiscover interface {
	IFaces() (string, error)
}

// NetworkChangeListener is called by Kotlin when the system reports a
// connectivity change (Wi-Fi -> cellular handoff, captive portal, etc).
// The Go engine reacts by re-resolving wg-cp0 endpoints and rotating
// the outer socket if needed.
type NetworkChangeListener interface {
	OnNetworkChanged(networkType string)
}

// DnsReadyListener fires when the engine has finished applying DNS
// settings to the system. Kotlin can use this to update UI; on Android
// the actual DNS configuration travels through VpnService.Builder so
// this is informational.
type DnsReadyListener interface {
	OnReady()
}

// PlatformFiles abstracts the per-app filesystem paths that the engine
// needs to persist state. The Kotlin shell supplies these via Android's
// Context.getFilesDir() / getCacheDir() — the engine cannot create files
// outside the app sandbox.
type PlatformFiles interface {
	// ConfigurationFilePath is where the imported bundle + derived
	// engine config live. Persisted across launches.
	ConfigurationFilePath() string
	// StateFilePath is engine ephemeral state (last handshake, etc).
	StateFilePath() string
	// CacheDir is for transient artifacts (debug bundles, log spool).
	CacheDir() string
}

// DNSList wraps a list of resolver IPs supplied by the bundle or the
// system. Exported as a struct (not a Go slice) so gomobile can bind it.
type DNSList struct {
	mu    sync.Mutex
	items []netip.AddrPort
}

// NewDNSList returns an empty DNSList. Kotlin calls Add() to populate.
func NewDNSList() *DNSList {
	return &DNSList{}
}

// Add appends a DNS resolver IP. Returns an error on parse failure.
// Default port is 53; callers can include ":port" for non-standard ports.
func (d *DNSList) Add(s string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if ap, err := netip.ParseAddrPort(s); err == nil {
		d.items = append(d.items, ap)
		return nil
	}
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return fmt.Errorf("invalid DNS address %q: %w", s, err)
	}
	d.items = append(d.items, netip.AddrPortFrom(addr.Unmap(), 53))
	return nil
}

// Get returns the i-th DNS resolver IP as a string.
func (d *DNSList) Get(i int) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if i < 0 || i >= len(d.items) {
		return "", fmt.Errorf("dns list index %d out of range", i)
	}
	return d.items[i].Addr().String(), nil
}

// Size returns the number of DNS resolvers.
func (d *DNSList) Size() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.items)
}

func (d *DNSList) snapshot() []netip.AddrPort {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]netip.AddrPort, len(d.items))
	copy(out, d.items)
	return out
}

// EnvList is a string→string map exported via gomobile so the Kotlin
// shell can pass tunable env vars (force-relay, lazy-conn thresholds)
// without committing to a CGO bridge.
type EnvList struct {
	mu   sync.Mutex
	data map[string]string
}

// NewEnvList returns an empty EnvList.
func NewEnvList() *EnvList {
	return &EnvList{data: make(map[string]string)}
}

// Put sets a key→value pair.
func (e *EnvList) Put(key, value string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.data[key] = value
}

// Get retrieves a value, "" if absent.
func (e *EnvList) Get(key string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.data[key]
}

func (e *EnvList) snapshot() map[string]string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make(map[string]string, len(e.data))
	for k, v := range e.data {
		out[k] = v
	}
	return out
}
