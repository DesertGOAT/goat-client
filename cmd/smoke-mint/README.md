# smoke-mint — mint a CA-signed enrollment bundle for local smokes

Constructs an `internal/bundle.EnrollmentBundle` with the supplied
WireGuard keypairs + endpoint, signs it with a supplied ECDSA P-256
PEM private key (PEM SEC1 or PKCS#8), and writes the canonical CBOR.
Companion to [`cmd/smoke-endpoint`](../smoke-endpoint/).

## Why this and not goat-trunk's `bundle-create`?

goat-trunk has a full-featured `ops/enrollment/cmd/bundle-create` that
handles age-encrypted CA keys, YubiKey-PIV signing, snitch-key
registration, and allowlist push-back. That's the right tool for
operational mints.

smoke-mint is the opposite end of the spectrum: a 100-line tool that
takes a plain PEM CA key and emits a bundle. Designed for fast local
iteration where you just want to validate the goat-client consumer
path against a known bundle shape, without touching any operational
infra.

## Trust-anchor caveat

Same as `cmd/smoke-endpoint` — the goat-client side embeds a fixed
trust anchor at build time (`internal/trustanchor.Default()`). A
bundle minted by smoke-mint will only verify if the goat-client was
built with a matching anchor. See
[`cmd/smoke-endpoint/README.md`](../smoke-endpoint/README.md#the-trust-anchor-gotcha)
for the temporary-swap workflow.

## Build

```bash
go build -o /tmp/smoke-mint ./cmd/smoke-mint
```

## Run

```bash
# Reuses the keypairs from the smoke-endpoint runbook
/tmp/smoke-mint \
  --ca-key   /tmp/smoke-ca.key.pem \
  --endpoint-pub  "$ENDPOINT_PUB" \
  --client-priv   "$CLIENT_PRIV" \
  --client-pub    "$CLIENT_PUB" \
  --endpoint-addr "10.0.2.2:51821" \
  --out      /tmp/smoke-bundle-android.cbor
```

Use `127.0.0.1:51821` for iOS Simulator and the desktop daemon;
`10.0.2.2:51821` for the Android emulator (its NAT route to the
host loopback).

The output `.cbor` is the file you `adb push` into Android storage
(or hand to the iOS document picker, or hand to `goat-clientd
importBundle`).

## What it puts in the bundle

- `DeviceID = "smoke-mobile-01"`, `Site = "smoke-lab"`,
  `ACLGroups = ["smoke"]`
- `IssuedAt = now`, `ActivationDeadline = +7d`, `ExpiresAt = +30d`
- A single `KnownEndpoint` with `Kind = "cp-relay"`,
  `Addr = <endpoint-addr>`, `MeshAddr = 198.18.0.2`,
  `Pubkey = <endpoint-pub>`
- `CPDeviceAddress = 198.18.0.100/24`, `CPDevicePubkey = <client-pub>`,
  `CPDevicePrivkey = <client-priv>`
- ECDSA P-256 ASN.1-DER signature over the canonical CBOR signable
  (matches the verifier in `internal/trustanchor`)

These values are hardcoded — if you need a different shape, edit
`main.go` directly. The tool is intentionally minimal.

## Lineage

Same as `cmd/smoke-endpoint`. Used during the 2026-05-12 session to
mint the two test bundles that drove the end-to-end Android emulator
handshake validating PR #32.
