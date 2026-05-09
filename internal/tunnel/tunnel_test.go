package tunnel

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/dlf-dds/goat-client/internal/bundle"
)

func TestFromBundle_HappyPath(t *testing.T) {
	priv := make([]byte, 32)
	pub := make([]byte, 32)
	for i := range priv {
		priv[i] = byte(i)
		pub[i] = byte(i + 1)
	}
	relayKey := make([]byte, 32)
	relayKey[0] = 0xAB
	b := &bundle.EnrollmentBundle{
		Version:         bundle.Version,
		DeviceID:        "dev-1",
		CPDevicePubkey:  pub,
		CPDevicePrivkey: priv,
		CPDeviceAddress: "198.18.0.6/24",
		KnownEndpoints: []bundle.KnownEndpoint{
			{Addr: "203.0.113.5:51820", Pubkey: relayKey, Kind: bundle.KindRelay, MeshAddr: "198.18.0.1"},
		},
	}
	cfg, err := FromBundle(b)
	if err != nil {
		t.Fatalf("FromBundle: %v", err)
	}
	if cfg.LocalAddress.String() != "198.18.0.6/24" {
		t.Errorf("LocalAddress: %q", cfg.LocalAddress)
	}
	if cfg.Peer.Endpoint != "203.0.113.5:51820" {
		t.Errorf("Endpoint: %q", cfg.Peer.Endpoint)
	}
	if len(cfg.Peer.AllowedIPs) != 1 || cfg.Peer.AllowedIPs[0].String() != "198.18.0.1/32" {
		t.Errorf("AllowedIPs: %v", cfg.Peer.AllowedIPs)
	}
	if cfg.Peer.PersistentKeepalive != 25*time.Second {
		t.Errorf("PersistentKeepalive: %v", cfg.Peer.PersistentKeepalive)
	}
}

func TestFromBundle_AllowedIPsOverride(t *testing.T) {
	priv := make([]byte, 32)
	pub := make([]byte, 32)
	relayKey := make([]byte, 32)
	b := &bundle.EnrollmentBundle{
		CPDevicePubkey:  pub,
		CPDevicePrivkey: priv,
		CPDeviceAddress: "198.18.0.6/24",
		KnownEndpoints: []bundle.KnownEndpoint{
			{Addr: "203.0.113.5:51820", Pubkey: relayKey, Kind: bundle.KindRelay,
				MeshAddr: "198.18.0.1", AllowedIPs: []string{"198.18.0.0/24"}},
		},
	}
	cfg, err := FromBundle(b)
	if err != nil {
		t.Fatalf("FromBundle: %v", err)
	}
	if len(cfg.Peer.AllowedIPs) != 1 || cfg.Peer.AllowedIPs[0].String() != "198.18.0.0/24" {
		t.Errorf("AllowedIPs override not applied: %v", cfg.Peer.AllowedIPs)
	}
}

func TestFromBundle_NoRelay(t *testing.T) {
	priv := make([]byte, 32)
	pub := make([]byte, 32)
	b := &bundle.EnrollmentBundle{
		CPDevicePubkey:  pub,
		CPDevicePrivkey: priv,
		CPDeviceAddress: "198.18.0.6/24",
		KnownEndpoints: []bundle.KnownEndpoint{
			{Kind: bundle.KindMgmt, Pubkey: make([]byte, 32)},
		},
	}
	if _, err := FromBundle(b); err != ErrNoEndpoint {
		t.Errorf("want ErrNoEndpoint, got %v", err)
	}
}

func TestBuildUAPI_SingleLineFormat(t *testing.T) {
	priv := make([]byte, 32)
	pub := make([]byte, 32)
	priv[0] = 1
	pub[0] = 2
	cfg := Config{
		PrivateKey: priv,
		Peer: PeerConfig{
			PublicKey:           pub,
			Endpoint:            "127.0.0.1:51820",
			AllowedIPs:          []netip.Prefix{netip.MustParsePrefix("198.18.0.1/32")},
			PersistentKeepalive: 25 * time.Second,
		},
		ListenPort: 0,
	}
	uapi, err := buildUAPI(cfg)
	if err != nil {
		t.Fatalf("buildUAPI: %v", err)
	}
	for _, want := range []string{"private_key=", "public_key=", "endpoint=127.0.0.1:51820",
		"persistent_keepalive_interval=25", "allowed_ip=198.18.0.1/32"} {
		if !strings.Contains(uapi, want) {
			t.Errorf("UAPI missing %q\nfull:\n%s", want, uapi)
		}
	}
}

func TestManagerStateMachine(t *testing.T) {
	m := NewManager()
	if got := m.State(); got != StateClosed {
		t.Errorf("initial state: %q", got)
	}
	priv := make([]byte, 32)
	pub := make([]byte, 32)
	if err := m.Configure(Config{
		InterfaceName: "wg-cp0-test",
		PrivateKey:    priv,
		Peer:          PeerConfig{PublicKey: pub},
	}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	// State stays Closed until Connect — Configure is config-only.
	if got := m.State(); got != StateClosed {
		t.Errorf("state after Configure: %q", got)
	}
}
