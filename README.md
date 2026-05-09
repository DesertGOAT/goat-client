# goat-client

Cross-platform daemon + GUI for goat **wg-cp0 silent control plane** onboarding.

Consumes an [offline-CA-signed CBOR bundle](https://github.com/dlf-dds/DesertBreadBird/blob/main/docs/design/offline-enrollment.md) and brings up + maintains the wg-cp0 outer WireGuard tunnel. Replaces the existing CLI ceremony (`bundle-create` operator-side / `wg-cp0-bundle-apply` device-side) with a friendly cross-platform app.

## Status

**Scaffolding (2026-05-09).** Foundation commit just landed. Build-out is happening in parallel via multiple Claude Code sessions per `HANDOFF.md`. **NOT shippable yet.**

## Platforms

- Linux (amd64 / arm64) — Fyne desktop GUI + Go daemon (kernel WireGuard or wireguard-go)
- macOS (amd64 / arm64) — Fyne desktop GUI + Go daemon (wireguard-go)
- Windows (amd64 / arm64) — Fyne desktop GUI + Go daemon (wireguard.dll)
- iOS / iPadOS — gomobile-built daemon framework + Swift NEPacketTunnelProvider shell
- Android — gomobile-built daemon AAR + Kotlin VpnService shell

## License

Apache 2.0 (see [LICENSE](LICENSE)). Forked from netbird's BSD-3-Clause-licensed `client/` tree at upstream commit `3fc5a8d4a1fe308ff1068764a09b90b0859ab8fe` (see [NOTICE](NOTICE) and [LICENSE.netbird-bsd3](LICENSE.netbird-bsd3) for attribution + license preservation).

## Build

```bash
go build ./...
```

(Currently builds the scaffolding stubs. Real build emerges as workstreams land.)

## Documentation

Authoritative design + ADR live in the goat trunk:

- [`docs/design/goat-client.md`](https://github.com/dlf-dds/DesertBreadBird/blob/main/docs/design/goat-client.md)
- [`docs/adr/0840-goat-client-cross-platform-daemon-gui.md`](https://github.com/dlf-dds/DesertBreadBird/blob/main/docs/adr/0840-goat-client-cross-platform-daemon-gui.md)
