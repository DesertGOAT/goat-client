package ipc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// stubClient is a self-contained, in-process Client implementation. It
// carries enough internal state to drive the GUI's state machine end-to-end
// (bundle import, connect, disconnect, status polling) before Track A's
// JSON-RPC daemon lands. It is NOT shared across processes — parent
// (systray) and child (window) instances each carry their own state.
type stubClient struct {
	addr string

	mu     sync.Mutex
	bundle *BundleInfo
	status StatusInfo
	logs   []string
}

func newStubClient(addr string) *stubClient {
	c := &stubClient{addr: addr}
	c.status.State = StateDisconnected
	c.appendLog("stub IPC client initialised; awaiting Track A daemon")
	return c
}

func (c *stubClient) ImportBundle(ctx context.Context, raw []byte) (*BundleInfo, error) {
	if len(raw) == 0 {
		return nil, errors.New("bundle is empty")
	}
	// Track A's bundle parser will replace this with real CBOR + Ed25519
	// verification. Stub returns a canned summary so the dialog has
	// something to display.
	now := time.Now().UTC()
	info := &BundleInfo{
		IssuedTo:   "stub-device",
		Site:       "lab-stub",
		NotBefore:  now.Add(-time.Hour),
		NotAfter:   now.Add(90 * 24 * time.Hour),
		PeerPubKey: "STUB+wg-cp0+peer+pubkey+base64==",
		Endpoints:  []string{"wg-cp0.example.invalid:51821"},
	}

	c.mu.Lock()
	c.bundle = info
	c.status.BundleImported = true
	c.status.Bundle = info
	c.appendLogLocked(fmt.Sprintf("imported bundle (%d bytes) — issued-to=%s site=%s", len(raw), info.IssuedTo, info.Site))
	c.mu.Unlock()
	return info, nil
}

func (c *stubClient) GetStatus(ctx context.Context) (*StatusInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	snapshot := c.status
	if c.bundle != nil {
		bcopy := *c.bundle
		snapshot.Bundle = &bcopy
	}
	if snapshot.State == StateConnected {
		snapshot.LastHandshake = time.Now().UTC().Add(-30 * time.Second)
		snapshot.BytesIn += 4096
		snapshot.BytesOut += 1024
	}
	return &snapshot, nil
}

func (c *stubClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.bundle == nil {
		c.mu.Unlock()
		return errors.New("no bundle imported")
	}
	c.status.State = StateConnecting
	c.status.ErrorMessage = ""
	c.appendLogLocked("connect requested → entering connecting state")
	c.mu.Unlock()

	go func() {
		select {
		case <-time.After(800 * time.Millisecond):
		case <-ctx.Done():
			return
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.status.State != StateConnecting {
			return
		}
		c.status.State = StateConnected
		c.status.InterfaceName = "wg-cp0"
		c.status.PeerPubKey = c.bundle.PeerPubKey
		c.status.Endpoints = append(c.status.Endpoints[:0], c.bundle.Endpoints...)
		c.appendLogLocked("tunnel up (stub)")
	}()
	return nil
}

func (c *stubClient) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status.State = StateDisconnected
	c.status.LastHandshake = time.Time{}
	c.status.BytesIn = 0
	c.status.BytesOut = 0
	c.appendLogLocked("disconnect requested → tunnel down")
	return nil
}

func (c *stubClient) GetDiagnostics(ctx context.Context) (*Diagnostics, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	tail := append([]string(nil), c.logs...)
	return &Diagnostics{
		LogTail:   tail,
		LastProbe: time.Now().UTC(),
		Reachable: c.status.State == StateConnected,
	}, nil
}

func (c *stubClient) Close() error {
	return nil
}

func (c *stubClient) appendLog(line string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.appendLogLocked(line)
}

func (c *stubClient) appendLogLocked(line string) {
	stamped := fmt.Sprintf("%s  %s", time.Now().UTC().Format(time.RFC3339), line)
	c.logs = append(c.logs, stamped)
	if len(c.logs) > 200 {
		c.logs = c.logs[len(c.logs)-200:]
	}
}
