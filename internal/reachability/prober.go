package reachability

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Prober probes a set of endpoints and returns a sorted []Result.
//
// Zero-value Prober is usable: defaults apply. Concurrent calls to
// Probe are safe — each call makes its own goroutines and does not
// share mutable state on the receiver.
type Prober struct {
	Timeout     time.Duration // per-probe timeout; 0 ⇒ DefaultTimeout
	MaxParallel int           // bounded concurrency; 0 ⇒ DefaultMaxParallel
	UDPPayload  []byte        // probe datagram for UDP endpoints; nil ⇒ defaultUDPPayload

	// Test seams. Unexported so the public API stays small.
	dial dialFunc
	now  func() time.Time
}

// New returns a Prober with default settings.
func New() *Prober { return &Prober{} }

func (p *Prober) timeout() time.Duration {
	if p.Timeout > 0 {
		return p.Timeout
	}
	return DefaultTimeout
}

func (p *Prober) maxParallel() int {
	if p.MaxParallel > 0 {
		return p.MaxParallel
	}
	return DefaultMaxParallel
}

func (p *Prober) udpPayload() []byte {
	if p.UDPPayload != nil {
		return p.UDPPayload
	}
	return defaultUDPPayload
}

func (p *Prober) dialer() dialFunc {
	if p.dial != nil {
		return p.dial
	}
	return defaultDial
}

func (p *Prober) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

// Probe runs one round against endpoints with bounded parallelism and
// returns the results sorted lowest-RTT-first. Unreachable endpoints
// sort to the end (their order among themselves preserves input order).
//
// The returned slice has the same length as endpoints. If endpoints is
// empty Probe returns nil.
func (p *Prober) Probe(ctx context.Context, endpoints []Endpoint) []Result {
	if len(endpoints) == 0 {
		return nil
	}

	results := make([]Result, len(endpoints))
	sem := make(chan struct{}, p.maxParallel())
	var wg sync.WaitGroup

	for i, ep := range endpoints {
		select {
		case <-ctx.Done():
			// Mark remaining endpoints as unreachable with the ctx error
			// rather than leaving them zero-valued. The probe round was
			// cancelled mid-flight; reflect that in the result.
			for j := i; j < len(endpoints); j++ {
				results[j] = Result{Endpoint: endpoints[j], Err: ctx.Err()}
			}
			wg.Wait()
			sortResults(results)
			return results
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(idx int, ep Endpoint) {
			defer wg.Done()
			defer func() { <-sem }()
			results[idx] = p.probeOne(ctx, ep)
		}(i, ep)
	}

	wg.Wait()
	sortResults(results)
	return results
}

// probeOne runs a single endpoint probe and returns its Result.
func (p *Prober) probeOne(ctx context.Context, ep Endpoint) Result {
	timeout := p.timeout()
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch ep.network() {
	case NetworkTCP:
		return p.probeTCP(probeCtx, ep, timeout)
	case NetworkUDP:
		return p.probeUDP(probeCtx, ep, timeout)
	default:
		return Result{Endpoint: ep, Err: fmt.Errorf("reachability: unknown network %q", ep.Network)}
	}
}

func (p *Prober) probeTCP(ctx context.Context, ep Endpoint, timeout time.Duration) Result {
	start := p.clock()
	conn, err := p.dialer()(ctx, NetworkTCP, ep.Address, timeout)
	rtt := p.clock().Sub(start)
	if err != nil {
		return Result{Endpoint: ep, Err: err}
	}
	_ = conn.Close()
	return Result{Endpoint: ep, Reachable: true, RTT: rtt}
}

func (p *Prober) probeUDP(ctx context.Context, ep Endpoint, timeout time.Duration) Result {
	conn, err := p.dialer()(ctx, NetworkUDP, ep.Address, timeout)
	if err != nil {
		return Result{Endpoint: ep, Err: err}
	}
	defer conn.Close()

	deadline := p.clock().Add(timeout)
	if err := conn.SetDeadline(deadline); err != nil {
		return Result{Endpoint: ep, Err: err}
	}

	start := p.clock()
	if _, err := conn.Write(p.udpPayload()); err != nil {
		return Result{Endpoint: ep, Err: err}
	}
	buf := make([]byte, 1500)
	if _, err := conn.Read(buf); err != nil {
		return Result{Endpoint: ep, Err: err}
	}
	return Result{Endpoint: ep, Reachable: true, RTT: p.clock().Sub(start)}
}

// sortResults orders by reachable-first, then by RTT ascending. Unreachable
// endpoints sort to the end and preserve their input order via stable sort.
func sortResults(rs []Result) {
	sort.SliceStable(rs, func(i, j int) bool {
		if rs[i].Reachable != rs[j].Reachable {
			return rs[i].Reachable
		}
		if !rs[i].Reachable {
			return false // ties among unreachable preserve input order
		}
		return rs[i].RTT < rs[j].RTT
	})
}

// Run probes endpoints once immediately, then re-probes every interval
// until ctx is done. It writes a snapshot to changes only when the
// reachability set or sort order has shifted relative to the previous
// snapshot — RTT-only fluctuations are filtered out so the daemon does
// not chase noise.
//
// Run blocks; callers spawn it as a goroutine. The first snapshot is
// always emitted (even if empty) so consumers see the initial state.
//
// changes may be nil (Run becomes a quiet warm-up loop) or buffered.
// On a full unbuffered channel Run drops the snapshot rather than
// blocking the probe cadence — the next cadence tick will emit the
// then-current state, which is what the daemon cares about.
func (p *Prober) Run(ctx context.Context, endpoints []Endpoint, interval time.Duration, changes chan<- []Result) {
	if interval <= 0 {
		interval = DefaultInterval
	}

	var prev string
	emit := func(rs []Result) {
		fp := fingerprint(rs)
		if fp == prev {
			return
		}
		prev = fp
		if changes == nil {
			return
		}
		select {
		case changes <- rs:
		case <-ctx.Done():
		default:
			// Channel full; skip this snapshot. The next tick emits.
		}
	}

	// Initial probe. Force-emit by setting prev to a sentinel that
	// fingerprint() never produces.
	prev = "\x00uninit"
	emit(p.Probe(ctx, endpoints))

	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			emit(p.Probe(ctx, endpoints))
		}
	}
}

// fingerprint produces a stable identity for a result slice based on
// (sorted) address-and-reachability sequence. Ignores RTT.
func fingerprint(rs []Result) string {
	var b strings.Builder
	for i, r := range rs {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(r.Endpoint.network())
		b.WriteByte('|')
		b.WriteString(r.Endpoint.Address)
		b.WriteByte('|')
		if r.Reachable {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	}
	return b.String()
}

