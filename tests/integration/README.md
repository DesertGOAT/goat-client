# Integration tests — goat-client

End-to-end tests that build `goat-clientd` and drive it through its IPC
surface (the `Client` interface in [`internal/ipc/types.go`](../../internal/ipc/types.go)).
Two tiers, gated by Go build tags.

## Tier-A — `integration` build tag

Hermetic. The test process:

1. Mints a fresh Ed25519 trust-root keypair, writes the public key as a
   PEM file.
2. Constructs a valid `bundle.EnrollmentBundle` (timestamps, device ID,
   site, one relay endpoint), signs with the private key, marshals to
   canonical CBOR.
3. Builds `cmd/goat-clientd` (cached after first test).
4. Spawns the daemon with `--socket <short-temp> --trust-roots <pem>
   --bundle <state-path>`.
5. Dials via `ipc.NewClient("unix://...")`.
6. Exercises:
   - `GetStatus` pre-import → `BundleImported=false`
   - `ImportBundle(valid)` → returns `BundleInfo` matching fixture
   - `ImportBundle(empty)` → error
   - `ImportBundle(foreign-signed)` → error mentioning signature/trust/verify
   - `Connect` pre-import → error mentioning bundle
   - `GetDiagnostics` → non-empty `LogTail`
   - Persistence across restart → second daemon spawn against same
     `--bundle` path picks up persisted state via `LoadPersistedBundle`

`Connect` post-import is **not** exercised at this tier — Track A's
daemon brings up real wg-cp0 via `wireguard-go` which needs
`CAP_NET_ADMIN` / a real TUN device. CI runners don't have that; that
coverage lives in the realprotocol sibling.

Run locally:

```bash
go test -tags integration -count=1 -v ./tests/integration/...
```

Wall-clock budget: under 30s for the full class.

## Tier-B — `realprotocol` build tag (sibling)

Spawns the daemon, imports a real CBOR-+-Ed25519 bundle minted by the
offline-CA workstation, brings the wg-cp0 outer tunnel up against a
live endpoint in the goat sandbox lab, asserts handshake within the
deadline. Skipped unless `GOAT_LAB_BUNDLE_PATH` *and*
`GOAT_LAB_TRUST_ROOTS_PATH` are set.

Required env:

| Var | Purpose |
|---|---|
| `GOAT_LAB_BUNDLE_PATH` | path to a CBOR-encoded offline-CA bundle valid for a wg-cp0 endpoint in the lab |
| `GOAT_LAB_TRUST_ROOTS_PATH` | PEM file with the Ed25519 pubkey that signed the bundle |

Optional env:

| Var | Default | Purpose |
|---|---|---|
| `GOAT_LAB_TIMEOUT_SEC` | 30 | handshake deadline |

Run locally (assuming you have lab access):

```bash
export GOAT_LAB_BUNDLE_PATH=/path/to/lab.bundle.cbor
export GOAT_LAB_TRUST_ROOTS_PATH=/path/to/lab-trust-roots.pem
go test -tags realprotocol -count=1 -v ./tests/integration/...
```

CI: nightly job + on changes to `internal/tunnel/**` or
`internal/bundle/**`.

## Sibling pattern

Lifted from goat-trunk's
[Block 50G/I real-protocol e2e harness](https://github.com/dlf-dds/DesertBreadBird/blob/main/docs/design/real-protocol-e2e-validation.md).
Same Tier-A-+-Tier-B split: in-process / hermetic on every PR;
live real-protocol nightly + on adjacent code changes.
