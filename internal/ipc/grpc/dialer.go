// Package grpc carries the goat-prod mgmt-API gRPC dialer: TLS validation
// against a goat-offline-CA root that's embedded at compile time, plus the
// ServerName-port-strip patch needed because the offline-CA service certs
// carry IP-SANs (not DNS-SANs).
//
// Lifted from netbird@32d04da19a's client/grpc/dialer.go — that file
// already carries the patch we authored upstream. The lift drops netbird's
// embeddedroots fallback (Mozilla CA bundle, used when SystemCertPool
// fails) and the WithCustomDialer hook (mesh-overlay-specific dial
// machinery) because goat-client doesn't need either: SystemCertPool is
// reliable on the desktop targets, and our wg-cp0 dialer is the OS
// default. The remaining surface is the offline-CA root injection + the
// IP-SAN-friendly tls.Config that the mgmt-API consumer will use.
//
// Note: Track A does not yet have a goat-clientd code path that talks
// gRPC to a mgmt-API endpoint — this package is pre-wired plumbing so
// the eventual control-plane RPC integration (snitch app, peer
// reconciliation, CRL pull) doesn't have to re-author the TLS trust path.
package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"fmt"
	"net"
	"runtime"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// goatOfflineCARoot is the offline-CA root cert (Block 22J / ADR 0407) that
// signs all goat-prod mgmt-API service certs. macOS Security framework
// rejects the Ed25519 root from the System keychain (`Unknown format in
// import`), so the daemon's TLS verify needs an in-binary trust anchor to
// validate `https://198.18.0.1:443` (kwt-aj-A) and equivalent per-site
// mgmt endpoints.
//
//go:embed dogfood-trust/goat-offline-ca-root.pem
var goatOfflineCARoot []byte

// CreateConnection creates a gRPC client connection that trusts the goat
// offline-CA root for TLS. Passing tlsEnabled=false drops to insecure
// credentials (test-only).
func CreateConnection(ctx context.Context, addr string, tlsEnabled bool, extraOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	transportOption := grpc.WithTransportCredentials(insecure.NewCredentials())
	if tlsEnabled && runtime.GOOS != "js" {
		certPool, err := x509.SystemCertPool()
		if err != nil || certPool == nil {
			certPool = x509.NewCertPool()
		}
		// Always add the goat offline-CA root as a trust anchor so
		// goat-prod mgmt endpoints (https://198.18.0.X:443, signed by
		// the offline-CA per ADR 0407) validate without requiring a
		// per-laptop macOS keychain entry that's blocked by Ed25519.
		if ok := certPool.AppendCertsFromPEM(goatOfflineCARoot); !ok {
			return nil, fmt.Errorf("goat offline-CA root failed to parse")
		}

		// gRPC's default ServerName-from-target behaviour passes
		// "host:port" to the verifier, which fails IP-SAN match (Go's
		// Certificate.VerifyHostname tries to parse "198.18.0.1:443" as
		// IP, which fails). Strip the port so IP-SAN match works.
		serverName := addr
		if h, _, splitErr := net.SplitHostPort(addr); splitErr == nil {
			serverName = h
		}
		transportOption = grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			RootCAs:    certPool,
			ServerName: serverName,
		}))
	}

	connCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	opts := []grpc.DialOption{
		transportOption,
		grpc.WithBlock(),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    30 * time.Second,
			Timeout: 10 * time.Second,
		}),
	}
	opts = append(opts, extraOpts...)

	conn, err := grpc.DialContext(connCtx, addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("dial context: %w", err)
	}
	return conn, nil
}
