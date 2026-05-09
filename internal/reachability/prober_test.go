package reachability

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// tcpResponder spins up a localhost listener that accepts then immediately
// closes. Returns the host:port and a cleanup func.
func tcpResponder(t *testing.T) (string, func()) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	return l.Addr().String(), func() {
		_ = l.Close()
		<-done
	}
}

// tcpRefused returns an address that should reject TCP connects: bind a
// listener, capture its port, then close it before returning. The kernel
// frees the port; connecting will get RST → "connection refused".
func tcpRefused(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// udpEcho spins up a UDP listener that echoes any datagram back to its
// sender after an optional delay. Returns the host:port and a cleanup.
func udpEcho(t *testing.T, delay time.Duration) (string, func()) {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("udp listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 1500)
		for {
			n, peer, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			if delay > 0 {
				time.Sleep(delay)
			}
			_, _ = conn.WriteTo(buf[:n], peer)
		}
	}()
	return conn.LocalAddr().String(), func() {
		_ = conn.Close()
		<-done
	}
}

// udpDrop spins up a UDP listener that reads but never replies.
func udpDrop(t *testing.T) (string, func()) {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("udp listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 1500)
		for {
			if _, _, err := conn.ReadFrom(buf); err != nil {
				return
			}
		}
	}()
	return conn.LocalAddr().String(), func() {
		_ = conn.Close()
		<-done
	}
}

func TestEndpointsUDP(t *testing.T) {
	got := EndpointsUDP([]string{"a:1", "b:2"})
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	for _, ep := range got {
		if ep.Network != NetworkUDP {
			t.Errorf("network=%q want %q", ep.Network, NetworkUDP)
		}
	}
}

func TestProbeTCPResponds(t *testing.T) {
	addr, cleanup := tcpResponder(t)
	defer cleanup()

	p := New()
	got := p.Probe(context.Background(), []Endpoint{{Network: NetworkTCP, Address: addr}})
	if len(got) != 1 {
		t.Fatalf("len=%d want 1", len(got))
	}
	if !got[0].Reachable {
		t.Fatalf("expected reachable, got err=%v", got[0].Err)
	}
	if got[0].RTT < 0 {
		t.Errorf("rtt=%v want >= 0", got[0].RTT)
	}
}

func TestProbeTCPRefused(t *testing.T) {
	addr := tcpRefused(t)

	p := &Prober{Timeout: 500 * time.Millisecond}
	got := p.Probe(context.Background(), []Endpoint{{Network: NetworkTCP, Address: addr}})
	if got[0].Reachable {
		t.Fatalf("expected unreachable, got reachable result")
	}
	if got[0].Err == nil {
		t.Fatalf("expected non-nil err on unreachable")
	}
}

func TestProbeUDPResponds(t *testing.T) {
	addr, cleanup := udpEcho(t, 0)
	defer cleanup()

	p := &Prober{Timeout: 500 * time.Millisecond}
	got := p.Probe(context.Background(), []Endpoint{{Network: NetworkUDP, Address: addr}})
	if !got[0].Reachable {
		t.Fatalf("expected reachable, got err=%v", got[0].Err)
	}
}

func TestProbeUDPDrops(t *testing.T) {
	addr, cleanup := udpDrop(t)
	defer cleanup()

	p := &Prober{Timeout: 100 * time.Millisecond}
	start := time.Now()
	got := p.Probe(context.Background(), []Endpoint{{Network: NetworkUDP, Address: addr}})
	elapsed := time.Since(start)
	if got[0].Reachable {
		t.Fatalf("expected unreachable on UDP drop")
	}
	// Must respect the timeout — give some slack for scheduling.
	if elapsed > 1*time.Second {
		t.Errorf("probe ran %v, expected ~100ms (timeout)", elapsed)
	}
}

func TestProbeSortsLowestRTTFirst(t *testing.T) {
	fast, c1 := udpEcho(t, 0)
	defer c1()
	slow, c2 := udpEcho(t, 80*time.Millisecond)
	defer c2()
	dropAddr, c3 := udpDrop(t)
	defer c3()

	p := &Prober{Timeout: 300 * time.Millisecond}
	results := p.Probe(context.Background(), []Endpoint{
		{Network: NetworkUDP, Address: slow},
		{Network: NetworkUDP, Address: dropAddr},
		{Network: NetworkUDP, Address: fast},
	})

	if len(results) != 3 {
		t.Fatalf("len=%d want 3", len(results))
	}
	if results[0].Endpoint.Address != fast {
		t.Errorf("first=%q want %q (fast)", results[0].Endpoint.Address, fast)
	}
	if results[1].Endpoint.Address != slow {
		t.Errorf("second=%q want %q (slow)", results[1].Endpoint.Address, slow)
	}
	if results[2].Endpoint.Address != dropAddr {
		t.Errorf("third=%q want %q (drop)", results[2].Endpoint.Address, dropAddr)
	}
	if !results[0].Reachable || !results[1].Reachable {
		t.Errorf("expected first two reachable")
	}
	if results[2].Reachable {
		t.Errorf("expected drop result unreachable")
	}
	if !(results[0].RTT < results[1].RTT) {
		t.Errorf("rtt order: %v should be < %v", results[0].RTT, results[1].RTT)
	}
}

// TestProbeBoundedParallelism verifies MaxParallel caps in-flight probes.
// We inject a dialFunc that bumps a counter on entry and asserts the peak
// never exceeds MaxParallel.
func TestProbeBoundedParallelism(t *testing.T) {
	const n = 8
	const limit = 3

	var inFlight, peak int32
	var mu sync.Mutex
	dial := func(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
		now := atomic.AddInt32(&inFlight, 1)
		defer atomic.AddInt32(&inFlight, -1)
		mu.Lock()
		if now > peak {
			peak = now
		}
		mu.Unlock()
		// Hold the slot long enough that probes overlap.
		select {
		case <-time.After(50 * time.Millisecond):
		case <-ctx.Done():
		}
		return nil, errors.New("synthetic: not really dialed")
	}

	p := &Prober{MaxParallel: limit, Timeout: time.Second, dial: dial}
	eps := make([]Endpoint, n)
	for i := range eps {
		eps[i] = Endpoint{Network: NetworkTCP, Address: "127.0.0.1:1"}
	}
	_ = p.Probe(context.Background(), eps)

	if peak == 0 {
		t.Fatalf("dialer never called")
	}
	if peak > limit {
		t.Fatalf("peak in-flight=%d exceeds limit=%d", peak, limit)
	}
}

func TestProbeEmptyReturnsNil(t *testing.T) {
	if r := New().Probe(context.Background(), nil); r != nil {
		t.Errorf("expected nil for empty input, got %v", r)
	}
}

func TestProbeUnknownNetwork(t *testing.T) {
	p := New()
	got := p.Probe(context.Background(), []Endpoint{{Network: "sctp", Address: "127.0.0.1:1"}})
	if got[0].Reachable {
		t.Fatalf("unknown network should be unreachable")
	}
	if got[0].Err == nil {
		t.Fatalf("expected err for unknown network")
	}
}

// TestRunInitialEmit verifies that Run emits the first snapshot immediately.
func TestRunInitialEmit(t *testing.T) {
	addr, cleanup := udpEcho(t, 0)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changes := make(chan []Result, 4)
	p := &Prober{Timeout: 200 * time.Millisecond}
	go p.Run(ctx, []Endpoint{{Network: NetworkUDP, Address: addr}}, time.Hour, changes)

	select {
	case snap := <-changes:
		if len(snap) != 1 || !snap[0].Reachable {
			t.Fatalf("first snapshot wrong: %+v", snap)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no initial snapshot within 2s")
	}
}

// TestRunEmitsOnTransition verifies Run only emits when the reachable
// state or order changes — not on RTT-only fluctuations.
func TestRunEmitsOnTransition(t *testing.T) {
	// Start with a responder, then close it mid-test to force a transition.
	addr, cleanup := udpEcho(t, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changes := make(chan []Result, 8)
	p := &Prober{Timeout: 100 * time.Millisecond}
	go p.Run(ctx, []Endpoint{{Network: NetworkUDP, Address: addr}}, 80*time.Millisecond, changes)

	// First emission: reachable.
	first := waitForSnap(t, changes, 2*time.Second)
	if !first[0].Reachable {
		t.Fatalf("first snapshot expected reachable: %+v", first)
	}

	// Tear down the responder; subsequent probes should fail and produce
	// a single transition emission.
	cleanup()

	second := waitForSnap(t, changes, 2*time.Second)
	if second[0].Reachable {
		t.Fatalf("post-teardown snapshot expected unreachable: %+v", second)
	}

	// Drain briefly: no further emissions in the next ~250ms (state stable).
	select {
	case unexpected := <-changes:
		t.Fatalf("unexpected extra emission while state stable: %+v", unexpected)
	case <-time.After(250 * time.Millisecond):
	}
}

func waitForSnap(t *testing.T, ch <-chan []Result, within time.Duration) []Result {
	t.Helper()
	select {
	case s := <-ch:
		return s
	case <-time.After(within):
		t.Fatalf("no snapshot within %v", within)
		return nil
	}
}

// TestRunCancellation verifies Run returns when ctx is cancelled.
func TestRunCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	p := New()
	go func() {
		p.Run(ctx, nil, 50*time.Millisecond, nil)
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("Run did not return within 2s of ctx cancel")
	}
}

func TestSortStableForUnreachable(t *testing.T) {
	// Two unreachable in input order; verify they stay in input order.
	rs := []Result{
		{Endpoint: Endpoint{Address: "a"}, Reachable: false},
		{Endpoint: Endpoint{Address: "b"}, Reachable: true, RTT: 10 * time.Millisecond},
		{Endpoint: Endpoint{Address: "c"}, Reachable: false},
	}
	sortResults(rs)
	if rs[0].Endpoint.Address != "b" {
		t.Errorf("reachable should be first, got %q", rs[0].Endpoint.Address)
	}
	if rs[1].Endpoint.Address != "a" || rs[2].Endpoint.Address != "c" {
		t.Errorf("unreachable order should be a,c got %q,%q", rs[1].Endpoint.Address, rs[2].Endpoint.Address)
	}
}

func TestFingerprintIgnoresRTT(t *testing.T) {
	a := []Result{{Endpoint: Endpoint{Network: NetworkUDP, Address: "x:1"}, Reachable: true, RTT: 10 * time.Millisecond}}
	b := []Result{{Endpoint: Endpoint{Network: NetworkUDP, Address: "x:1"}, Reachable: true, RTT: 200 * time.Millisecond}}
	if fingerprint(a) != fingerprint(b) {
		t.Errorf("fingerprint should ignore RTT")
	}
	c := []Result{{Endpoint: Endpoint{Network: NetworkUDP, Address: "x:1"}, Reachable: false}}
	if fingerprint(a) == fingerprint(c) {
		t.Errorf("fingerprint should differ on reachability change")
	}
}
