# smoke-endpoint — userspace wg-cp0 peer for local handshake smokes

A minimal wireguard-go peer that listens on a configurable UDP port,
accepts a single peer (by pubkey + allowed_ips), and runs the standard
WireGuard handshake. Uses `tun/netstack` so no kernel interface is
created — entirely in-process. Useful for driving an end-to-end Connect
flow from a goat-client mobile build (or the desktop daemon) WITHOUT
needing access to the prod lab relays.

## Companion

[`cmd/smoke-mint`](../smoke-mint/) mints a CA-signed bundle that points
at this endpoint. Together they let you exercise the full
ImportBundle → tunnel.RunOnMobile → wgdevice → handshake path locally.

## When to use this

- **End-to-end Android emulator smoke** without lab access. The
  Android emulator reaches the host's loopback as `10.0.2.2`; bind
  smoke-endpoint on the host port, mint a bundle pointing at
  `10.0.2.2:<port>`, install the APK, tap Import + Connect, watch
  the handshake complete in this binary's logs.
- **iOS Simulator** — same idea (`127.0.0.1:<port>`), but note that
  Apple's NEPacketTunnelProvider is intentionally crippled on the
  Simulator. The handshake leg won't actually carry packets; only
  the bundle-import + cfg-derivation path is exercisable there.
- **Desktop daemon** local smoke without `GOAT_LAB_*` env.

## The trust-anchor gotcha

For the goat-client to accept a bundle signed by a test CA, the
embedded `internal/trustanchor.Default()` set must include the
matching pubkey. By default, only the production CA root is
embedded — bundles signed by a freshly generated test CA will fail
signature verification with
`trustanchor: signature not signed by any active anchor`.

Two ways around this for local-smoke purposes:

1. **Temporary anchor swap** (per-build):
   ```bash
   # 1) generate a test ECDSA P-256 CA
   openssl ecparam -name prime256v1 -genkey -noout -out /tmp/smoke-ca.key.pem
   openssl ec -in /tmp/smoke-ca.key.pem -pubout -out /tmp/smoke-ca.pub.pem

   # 2) patch internal/trustanchor/anchors.yaml — replace the production
   # anchor's public_key_pem with the contents of /tmp/smoke-ca.pub.pem
   #    (keep the name + validity window — only the key matters for
   #     signature verification)

   # 3) regenerate the embedded set
   go generate ./internal/trustanchor

   # 4) rebuild whatever you're testing (daemon, xcframework, AAR)

   # 5) revert anchors.yaml + regenerate before committing anything else
   git checkout -- internal/trustanchor/
   ```

2. **Future**: a build-tag-gated test-anchor set or an env-var override
   on `trustanchor.Default()` would make this self-contained. Not yet
   landed; would be a follow-up PR if this rig sees regular use.

## Build

```bash
go build -o /tmp/smoke-endpoint ./cmd/smoke-endpoint
```

## Run

```bash
ENDPOINT_PRIV=$(wg genkey)
ENDPOINT_PUB=$(echo "$ENDPOINT_PRIV" | wg pubkey)
CLIENT_PRIV=$(wg genkey)
CLIENT_PUB=$(echo "$CLIENT_PRIV" | wg pubkey)

/tmp/smoke-endpoint \
  --endpoint-priv "$ENDPOINT_PRIV" \
  --client-pub "$CLIENT_PUB" \
  --port 51821
```

Endpoint stays up until SIGINT. On handshake, it logs
`HANDSHAKE <RFC3339>` so a script can watch for the signal. Stats are
polled every 2s.

The `$CLIENT_PRIV`/`$CLIENT_PUB` pair go into the bundle minted by
`cmd/smoke-mint`; that bundle is then imported by the goat-client
side under test.

## Lineage

Originated as a session-time scratch tool while validating PR #32's
end-to-end Android emulator handshake (2026-05-12) without lab
access. The handshake-completion confirmation it provides (peer
"Received handshake initiation / Sending handshake response /
Receiving keepalive packet") is what proved the Kotlin singleton +
Go `wrap tun fd` fixes were working end-to-end through the JNI
bridge before merge.
