//go:build (linux && !android) || (darwin && !ios) || windows

package tunnel

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	wgdevice "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

// desktopTunnel is the Tunnel impl for Linux/macOS/Windows. It wraps the
// upstream golang.zx2c4.com/wireguard userspace device + cross-platform
// tun.CreateTUN. The userspace device works without CGO and on every
// desktop target — Linux kernel WG would be faster on that one platform,
// but Phase 1 ships userspace everywhere for code-path uniformity. A
// follow-up commit can add a Linux kernel-WG fast path via wgctrl-go.
type desktopTunnel struct {
	mu  sync.Mutex
	dev *wgdevice.Device
	t   tun.Device
	cfg Config

	stats     Stats
	lastStat  time.Time
	statErr   error
	closeOnce sync.Once
}

// newPlatformTunnel constructs the desktop Tunnel. Stages: create the TUN
// device, wrap it in a wireguard-go Device, leave it un-configured. The
// caller (Manager) follows with Configure + Up.
func newPlatformTunnel() (Tunnel, error) {
	return &desktopTunnel{}, nil
}

// open lazily creates the underlying TUN + wireguard-go device. We split
// this off NewTunnel so that Configure can be called before Up without
// holding device-open errors when no real bring-up is needed (tests).
func (d *desktopTunnel) open(name string, mtu int) error {
	if d.t != nil {
		return nil
	}
	t, err := tun.CreateTUN(name, mtu)
	if err != nil {
		return fmt.Errorf("create TUN %s: %w", name, err)
	}
	d.t = t
	logger := wgdevice.NewLogger(wgdevice.LogLevelError, fmt.Sprintf("(%s) ", name))
	d.dev = wgdevice.NewDevice(t, conn.NewDefaultBind(), logger)
	return nil
}

// Configure applies a single-peer config to the wireguard-go device using
// its UAPI. UAPI is wireguard-go's text-config interface; same shape as
// `wg setconf`. We construct it from cfg and pipe it through device.IpcSet.
func (d *desktopTunnel) Configure(cfg Config) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.open(cfg.InterfaceName, int(cfg.MTU)); err != nil {
		return err
	}
	uapi, err := buildUAPI(cfg)
	if err != nil {
		return err
	}
	if err := d.dev.IpcSet(uapi); err != nil {
		return fmt.Errorf("apply UAPI config: %w", err)
	}
	d.cfg = cfg
	return nil
}

// Up brings the device's link state up. wireguard-go's Device starts its
// goroutines on construction; Up is the moment we apply per-platform
// link-up + address + DNS. The address/route/DNS plumbing is delegated to
// platform helpers in tunnel_*.go (linux, darwin, windows).
func (d *desktopTunnel) Up(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.dev == nil {
		return errors.New("tunnel: not configured")
	}
	if err := d.dev.Up(); err != nil {
		return fmt.Errorf("device up: %w", err)
	}
	if err := platformAssignAddress(d.cfg.InterfaceName, d.cfg.LocalAddress); err != nil {
		return fmt.Errorf("assign address: %w", err)
	}
	for _, p := range d.cfg.Peer.AllowedIPs {
		if err := platformAddRoute(d.cfg.InterfaceName, p); err != nil {
			return fmt.Errorf("add route %s: %w", p, err)
		}
	}
	return nil
}

// Down brings the device's link state down. Address/route teardown is
// best-effort — we log but don't fail Down on cleanup errors so a failed
// reconnect doesn't leave the daemon stuck in StateError.
func (d *desktopTunnel) Down(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.dev == nil {
		return nil
	}
	for _, p := range d.cfg.Peer.AllowedIPs {
		if err := platformDelRoute(d.cfg.InterfaceName, p); err != nil {
			log.Printf("tunnel: delete route %s: %v", p, err)
		}
	}
	if err := d.dev.Down(); err != nil {
		return fmt.Errorf("device down: %w", err)
	}
	return nil
}

// Stats reads counters from the device's UAPI get. Cached for 1s so a
// chatty getStatus poll doesn't hit the device per-call.
func (d *desktopTunnel) Stats() (Stats, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.dev == nil {
		return Stats{}, nil
	}
	if time.Since(d.lastStat) < time.Second {
		return d.stats, d.statErr
	}
	stats, err := readUAPIStats(d.dev)
	d.stats = stats
	d.lastStat = time.Now()
	d.statErr = err
	return stats, err
}

func (d *desktopTunnel) Close() error {
	var err error
	d.closeOnce.Do(func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		if d.dev != nil {
			d.dev.Close()
		}
		// device.Close() takes ownership of the underlying tun.Device,
		// so we don't separately Close d.t here.
		d.dev = nil
		d.t = nil
	})
	return err
}

// buildUAPI renders the cfg to wireguard-go's UAPI text format. UAPI keys
// are documented in wireguard.com/xplatform/. The format is line-oriented
// with `key=value` pairs and a blank line as terminator; IpcSet expects a
// single block (no trailing blank line in our case — IpcSet handles both
// shapes).
func buildUAPI(cfg Config) (string, error) {
	if len(cfg.PrivateKey) != 32 {
		return "", errors.New("private key not 32 bytes")
	}
	if len(cfg.Peer.PublicKey) != 32 {
		return "", errors.New("peer public key not 32 bytes")
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "private_key=%s\n", hex.EncodeToString(cfg.PrivateKey))
	if cfg.ListenPort > 0 {
		fmt.Fprintf(&sb, "listen_port=%d\n", cfg.ListenPort)
	}
	sb.WriteString("replace_peers=true\n")
	fmt.Fprintf(&sb, "public_key=%s\n", hex.EncodeToString(cfg.Peer.PublicKey))
	if cfg.Peer.Endpoint != "" {
		host, port, err := net.SplitHostPort(cfg.Peer.Endpoint)
		if err != nil {
			return "", fmt.Errorf("split endpoint %q: %w", cfg.Peer.Endpoint, err)
		}
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			return "", fmt.Errorf("resolve endpoint host %q: %w", host, err)
		}
		fmt.Fprintf(&sb, "endpoint=%s\n", net.JoinHostPort(ips[0].String(), port))
	}
	if cfg.Peer.PersistentKeepalive > 0 {
		fmt.Fprintf(&sb, "persistent_keepalive_interval=%d\n", int(cfg.Peer.PersistentKeepalive.Seconds()))
	}
	sb.WriteString("replace_allowed_ips=true\n")
	for _, p := range cfg.Peer.AllowedIPs {
		fmt.Fprintf(&sb, "allowed_ip=%s\n", p.String())
	}
	return sb.String(), nil
}

// readUAPIStats issues an IpcGet against the device and parses the
// response into Stats. UAPI returns `tx_bytes`, `rx_bytes`,
// `last_handshake_time_sec` (and _nsec) per peer.
func readUAPIStats(dev *wgdevice.Device) (Stats, error) {
	pr, pw := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		defer pw.Close()
		errCh <- dev.IpcGetOperation(pw)
	}()
	var stats Stats
	scanner := newLineScanner(pr)
	for scanner.Scan() {
		k, v, ok := splitKV(scanner.Text())
		if !ok {
			continue
		}
		switch k {
		case "tx_bytes":
			stats.BytesOut, _ = strconv.ParseUint(v, 10, 64)
		case "rx_bytes":
			stats.BytesIn, _ = strconv.ParseUint(v, 10, 64)
		case "last_handshake_time_sec":
			s, _ := strconv.ParseInt(v, 10, 64)
			if s > 0 {
				stats.LastHandshake = time.Unix(s, 0)
			}
		}
	}
	if err := <-errCh; err != nil {
		return stats, fmt.Errorf("ipc get: %w", err)
	}
	return stats, nil
}

func splitKV(line string) (string, string, bool) {
	eq := strings.IndexByte(line, '=')
	if eq <= 0 {
		return "", "", false
	}
	return line[:eq], line[eq+1:], true
}
