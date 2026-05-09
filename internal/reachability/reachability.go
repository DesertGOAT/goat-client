// Package reachability probes the wg-cp0 relay endpoints carried in a
// bundle and reports per-endpoint reachability + RTT, lowest-latency-first.
//
// The package is a pure library: no IPC, no daemon state, no goroutines
// owned outside what callers explicitly start via Run. The daemon's
// tunnel manager (Track A, internal/tunnel) consumes a []Result snapshot
// to pick which wg-cp0 relay to dial; this package gives it a sorted
// candidate list and re-emits when reachability changes. The integration
// seam is the []Result return value — there is no shared state and no
// import cycle with internal/tunnel.
//
// Probe semantics, per ADR 0840 + design doc §3:
//
//   - TCP endpoints: TCP connect with a per-probe timeout. Connect
//     success ⇒ reachable; refused / timed-out / unreachable ⇒ not.
//   - UDP endpoints: connected UDP socket; write a small probe payload
//     (a minimal DNS-style header by default), wait for any reply within
//     the timeout. Any datagram from the peer counts as reachable. This
//     matches the design-doc "UDP echo / DNS-test" probe shape; for real
//     wg-cp0 (a silent listener) the daemon may layer a WireGuard
//     handshake-init probe on top of this primitive — that lives in
//     internal/tunnel, not here.
//
// Endpoints are probed concurrently with bounded parallelism (default 5)
// so that a probe of 50 candidates does not open 50 sockets at once.
package reachability

import (
	"context"
	"net"
	"time"
)

// Default tunables. Callers override on the Prober struct.
const (
	DefaultTimeout     = 3 * time.Second
	DefaultMaxParallel = 5
	DefaultInterval    = 5 * time.Minute

	NetworkTCP = "tcp"
	NetworkUDP = "udp"
)

// Endpoint is a single probe target. Network is "tcp" or "udp"; empty
// defaults to "udp" (the wg-cp0 wire protocol).
type Endpoint struct {
	Network string
	Address string // host:port
}

func (e Endpoint) network() string {
	if e.Network == "" {
		return NetworkUDP
	}
	return e.Network
}

// Result is one endpoint's probe outcome. RTT is zero and Err is non-nil
// when Reachable is false.
type Result struct {
	Endpoint  Endpoint
	Reachable bool
	RTT       time.Duration
	Err       error
}

// EndpointsUDP wraps a bundle's []string endpoint list (host:port) into
// the UDP-defaulted Endpoint shape the prober expects. wg-cp0 is UDP, so
// this is the typical bundle → prober adaptor.
func EndpointsUDP(addrs []string) []Endpoint {
	out := make([]Endpoint, len(addrs))
	for i, a := range addrs {
		out[i] = Endpoint{Network: NetworkUDP, Address: a}
	}
	return out
}

// defaultUDPPayload is a minimal DNS query header (12 bytes, ID=0,
// recursion-desired) used as the default UDP probe datagram. We don't
// expect or parse a DNS reply — we just want any datagram back as a
// reachability signal. Production probes against wg-cp0 will be replaced
// by the daemon with a WireGuard handshake-init probe; this default
// keeps the standalone package useful for generic UDP probing and tests.
var defaultUDPPayload = []byte{
	0x00, 0x00, // ID
	0x01, 0x00, // flags: standard query, recursion desired
	0x00, 0x00, // QDCOUNT
	0x00, 0x00, // ANCOUNT
	0x00, 0x00, // NSCOUNT
	0x00, 0x00, // ARCOUNT
}

// dialFunc is the seam used by tests to stub network behaviour.
type dialFunc func(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error)

func defaultDial(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
	d := net.Dialer{Timeout: timeout}
	return d.DialContext(ctx, network, address)
}
