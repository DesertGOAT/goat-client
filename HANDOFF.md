# goat-client build-out — HANDOFF for parallel worker sessions

**Captain:** the operator's primary Claude Code session in `dlf-dds/DesertBreadBird` (this is one). Integrates worker output and lands milestones.

**Workers:** N additional Claude Code sessions opened in parallel (one VSCode window per worker). Each picks up exactly **one** track below via `/iso enter <track>`. Tracks run concurrently — no track depends on another track's mid-flight state, only on the foundation commit that landed this scaffolding.

**Working tree convention:** workers `cd /Users/dene/src/github.com/dlf-dds/goat-client` and provision a worktree per track (per the file-level master-worktree-readonly invariant codified in goat-trunk ADR 0013 Amendment 2026-05-09). All Edit/Write target the worktree path, never the master goat-client checkout.

**Source of truth — what to fork:** netbird upstream pinned at `3fc5a8d4a1fe308ff1068764a09b90b0859ab8fe`. Local fork at `/Users/dene/src/github.com/dfarrel1/netbird/` (one extra commit `32d04da19` carrying the embed-CA + ServerName-port-strip patch — already adopted in our `client/grpc/` fork target).

**Authoritative design + ADR:** read these FIRST before starting any track:
- `https://github.com/dlf-dds/DesertBreadBird/blob/main/docs/design/goat-client.md`
- `https://github.com/dlf-dds/DesertBreadBird/blob/main/docs/adr/0840-goat-client-cross-platform-daemon-gui.md`

---

## Track A — desktop spine: tunnel manager + bundle import + IPC

**Track name:** `goat-client-desktop-spine`
**Branch:** `track/desktop-spine`
**Estimated time:** 5-8 days single worker
**Blocks:** nothing (foundational); blocks tracks E + F + G downstream

**What to do:**

1. Fork `client/iface/` from `~/src/github.com/dfarrel1/netbird/client/iface/` (KEEP per survey — gold WG iface mgmt) into `internal/tunnel/`. Strip multi-peer config loops; reshape `WGIface` interface for single-peer wg-cp0 (one tunnel, one remote endpoint, no UpdatePeer/RemovePeer churn).
2. Fork `client/internal/dns/host_*.go` (per-platform DNS adapters: systemd-resolved/scutil/NRPT/NEDNSSettings) into `internal/tunnel/dns/`. Strip mesh-DNS server (`server.go`, `tcpstack.go`, `upstream.go`).
3. Implement `internal/bundle/` — CBOR parse + Ed25519 signature verify against pinned offline-CA root. Reuse the parser library from goat-trunk `ops/enrollment/cmd/bundle-extract/` (it's a Go package; vendor / replicate).
4. Implement `internal/ipc/` — JSON-RPC over Unix socket (Linux/macOS) + named pipe (Windows). Local-uid auth on writes. Method set: `importBundle`, `getStatus`, `connect`, `disconnect`, `getDiagnostics`.
5. Wire `cmd/goat-clientd/main.go` to a real daemon: load bundle from `~/.goat-client/bundle.cbor`, raise tunnel via `internal/tunnel`, expose IPC.
6. **Acceptance:** `go build ./...` green for `linux/amd64` + `darwin/amd64` + `darwin/arm64` + `windows/amd64`. Manual smoke: bundle imported, tunnel up, ping a wg-cp0 peer.

**Files to fork from netbird (cite paths in commits):**
- `client/iface/iface.go`, `client/iface/device/`, `client/iface/iface_new_*.go` (per-platform)
- `client/internal/dns/host_unix.go`, `host_darwin.go`, `host_windows.go`, `host_ios.go`, `upstream_android.go`
- `client/grpc/dialer.go` (already carries embed-CA patch — copy as-is into `internal/ipc/grpc/`)

---

## Track B — Fyne desktop GUI

**Track name:** `goat-client-fyne-ui`
**Branch:** `track/fyne-ui`
**Estimated time:** 5-7 days single worker
**Blocks:** nothing direct; soft-blocked on Track A's IPC method set converging (can stub IPC client first)

**What to do:**

1. Fork `client/ui/client_ui.go` (Fyne main) + `client/ui/profile.go` + `client/ui/event_handler.go` + `client/ui/notifier/` + `client/ui/desktop/` + `client/ui/assets/` (system tray icons) from netbird. Drop into `internal/ui/`.
2. **STRIP entirely:** login/OAuth flows (`Login`, `WaitSSOLogin`, `showLoginURL`), networks/peers list, profile manager, SSH settings, account/email display in tray.
3. **ADD:** bundle-import dialog (drag-drop file + file-picker; show issued-to/site/expires/peer-pubkey/endpoints from parsed bundle; Apply button). Single-tunnel-status pane (interface state, last handshake, bytes in/out, peer pubkey, configured endpoints). Connect/Disconnect button. Diagnostics view (WG log tail, "test connection" button).
4. **KEEP:** systray menu structure (`fyne.io/systray`), tray-icon rotation (green/amber/red per design doc §3 of snitch-app.md — same convention), Fyne window/widget infrastructure, daemon-IPC client pattern (adapt RPC method set to Track A's).
5. **Acceptance:** `cmd/goat-client` (Fyne GUI) builds + launches on Linux + macOS + Windows; tray icon shows; bundle-import dialog works (even with stub IPC); system tray indicator changes color based on stubbed status.

**netbird paths to fork:**
- `client/ui/client_ui.go`
- `client/ui/profile.go` (reshape — bundle-list instead of profile-list)
- `client/ui/event_handler.go`
- `client/ui/notifier/`
- `client/ui/desktop/`
- `client/ui/assets/`

---

## Track C — iOS shell (NEPacketTunnelProvider + Swift)

**Track name:** `goat-client-ios-shell`
**Branch:** `track/ios-shell`
**Estimated time:** 1.5-2 weeks single worker (gates on Apple Developer Program for TestFlight, but TestFlight not required for engineering builds)
**Blocks:** nothing direct; soft-blocked on Track A's tunnel + bundle packages converging (gomobile expects them)

**What to do:**

1. Fork `client/ios/NetBirdSDK/client.go` (gomobile facade) into `mobile/ios/GoatClientSDK/`. Reshape: replace `Login` / `IsLoginRequired` methods with `ImportBundle(bundleBytes []byte) error` + `GetTunnelStatus() string`. The `Run(fd int32, interfaceName string, envList string) error` shape stays — Swift NEPacketTunnelProvider still passes the utun FD.
2. Author the Swift app shell in `mobile/ios/Shell/`:
   - Xcode project (`.xcodeproj` or SPM-based)
   - Main app: bundle-import via `UIDocumentPicker` + QR scan via `AVFoundation`; tunnel up/down button; status display
   - NetworkExtension target: `NEPacketTunnelProvider` subclass that loads the GoatClientSDK xcframework and calls `Run(fd, ...)` with the utun FD
   - App Group container for shared state between main app + NE extension
3. Build pipeline: `gomobile bind -target=ios -bundleid=io.dlf-dds.goat-client.framework -o GoatClientSDK.xcframework ./mobile/ios/GoatClientSDK` per netbird's `.github/workflows/mobile-build-validation.yml` pattern.
4. **Acceptance:** xcframework builds, Xcode project references it, app builds for iOS Simulator (no Apple Developer Program needed for simulator), bundle import + tunnel-up smoke runs end-to-end against a real wg-cp0 endpoint (use the sandbox lab's wg-cp0 tier).

**netbird paths to fork:**
- `client/ios/NetBirdSDK/client.go` (heavy reshape)
- `client/iface/device/device_ios.go`, `client/iface/iface_new_ios.go` (KEEP — utun FD bridge)
- `client/internal/dns/host_ios.go` (KEEP — NEDNSSettings)

**External reference (NOT in our local checkout, NOT being forked into goat-client):** `netbirdio/ios-client` (Apache 2.0 — verify before lifting). The NEPacketTunnelProvider Swift wiring there is the structural reference for our `mobile/ios/Shell/` even if we author from scratch.

---

## Track D — Android shell (VpnService + Kotlin)

**Track name:** `goat-client-android-shell`
**Branch:** `track/android-shell`
**Estimated time:** 1.5-2 weeks single worker (gates on Google Play developer account for Internal track, but Internal not required for sideloaded APK)
**Blocks:** nothing direct; soft-blocked on Track A's tunnel + bundle packages converging

**What to do:**

1. Fork `client/android/client.go` (gomobile facade) into `mobile/android/GoatClientSDK/`. Reshape: replace `Login*` methods with `ImportBundle(bundleBytes []byte) error` + `GetTunnelStatus() string`. The `Run(platformFiles, urlOpener, isAndroidTV, dns, dnsReadyListener, envList)` shape mostly stays; the `TunAdapter` interface (with `ConfigureInterface(addr, mtu, dns, routes) → fd`, `ProtectSocket(fd)`, `UpdateAddr()`) stays — Kotlin VpnService implements it.
2. Author the Kotlin app shell in `mobile/android/Shell/`:
   - Android Studio / Gradle project
   - Main activity: bundle-import via storage-access-framework + QR scan via CameraX; tunnel up/down button; status display
   - `VpnService` subclass that wraps the GoatClientSDK aar; implements `TunAdapter` (creates the VPN tunnel, passes FD to Go via `ConfigureInterface`)
   - Foreground service for active session; persistent notification with status
3. Build pipeline: `gomobile bind -target=android -javapkg=io.dlf-dds.goat_client.gomobile -o goat-client.aar ./mobile/android/GoatClientSDK` per netbird's pattern.
4. **Acceptance:** AAR builds, Gradle project references it, APK builds + sideloads to Android emulator, bundle import + tunnel-up smoke runs end-to-end.

**netbird paths to fork:**
- `client/android/client.go` (heavy reshape)
- `client/iface/device/device_android.go`, `client/iface/iface_new_android.go` (KEEP — VpnService bridge)
- `client/net/protectsocket_android.go` (KEEP — Android socket protection)

**External reference:** `netbirdio/android-client`. Same caveat as Track C.

---

## Track E — five-platform CI matrix + cosign-signed releases

**Track name:** `goat-client-ci-matrix`
**Branch:** `track/ci-matrix`
**Estimated time:** 2-3 days single worker
**Blocks:** nothing; can run alongside any other track from day 1

**What to do:**

1. Author `.github/workflows/release.yml` mirroring goat-trunk's [Block 61H snitch CI pattern](https://github.com/dlf-dds/DesertBreadBird/blob/main/.github/workflows/snitch.yml). Six desktop targets: `linux/{amd64,arm64}`, `darwin/{amd64,arm64}`, `windows/{amd64,arm64}`. CGO_ENABLED=0 throughout.
2. Reproducible build flags: `-trimpath -buildvcs=false`. Per-asset `.sha256` + aggregate `SHA256SUMS`. Cosign-signed binaries on tag push (`goat-client-v<semver>`).
3. Mobile build validation (advisory, doesn't gate desktop release): mirror netbird's `.github/workflows/mobile-build-validation.yml` for `gomobile bind` smoke against `mobile/ios/GoatClientSDK` + `mobile/android/GoatClientSDK`.
4. Tier-A always-fast CI: `go vet ./... && go test ./... && go build ./...` on every PR + push to main.
5. **Acceptance:** A test tag `goat-client-v0.0.1-pre` produces 6 binaries + 6 .sha256 + SHA256SUMS + cosign signatures attached to GitHub Release.

---

## Track F — per-platform desktop packaging

**Track name:** `goat-client-packaging`
**Branch:** `track/packaging`
**Estimated time:** 3-5 days single worker
**Blocks:** soft-blocked on Track A (need a real binary to package); can author packaging skeletons immediately

**What to do:**

1. `packaging/deb/` — Debian/Ubuntu `.deb` package definition. systemd unit installs `goat-clientd` as a system service. GUI binary + .desktop launcher for per-user app.
2. `packaging/rpm/` — Fedora/RHEL `.rpm` package, parallel structure.
3. `packaging/dmg/` — macOS .dmg builder. launchd LaunchDaemon for daemon. .app bundle for GUI. (Apple Developer ID notarization gates stable release; engineering builds ship unsigned.)
4. `packaging/msi/` — Windows MSI builder (WiX or similar; netbird uses NSIS — see `~/src/github.com/dfarrel1/netbird/installer.nsis` + `netbird.wxs`). Windows Service for daemon. Authenticode signing operator-fired procurement; engineering builds ship unsigned.
5. **Acceptance:** one install/uninstall round-trip per platform on CI runners (Linux apt + rpm-test container, macOS runner, Windows runner); daemon auto-starts at boot; GUI launches.

---

## Track G — bundle-import IPC contract + integration test

**Track name:** `goat-client-bundle-ipc-test`
**Branch:** `track/bundle-ipc-test`
**Estimated time:** 2-3 days single worker
**Blocks:** depends on Track A's IPC method set + Track B's GUI bundle-import dialog converging (parallel-with-tracks-A-and-B-as-they-stabilize)

**What to do:**

1. Author end-to-end integration test: spin up a `goat-clientd` (Track A binary) via testcontainers-go OR direct exec in CI; have a stub Fyne client (or just an IPC test client) call `importBundle` → verify daemon's tunnel state + persisted config; then call `connect` → verify wg-cp0 tunnel goes up against a mock-or-real endpoint; `disconnect` → tunnel down.
2. Sibling: a real-protocol test against a live wg-cp0 endpoint in the goat sandbox lab (same shape as goat-trunk's [Block 50I real-protocol e2e](https://github.com/dlf-dds/DesertBreadBird/blob/main/docs/design/real-protocol-e2e-validation.md) — fills correctness-under-modest-concurrency tier between unit and soak).
3. **Acceptance:** integration test runs in CI on every PR; real-protocol test runs nightly + on tunnel-package changes.

---

## Cross-track coordination

**Per-track branches push to dlf-dds/goat-client and open PRs.** PRs target main; squash-merge per goat-trunk convention. CODEOWNERS not yet authored (Track E can land it).

**Captain (this session in goat-trunk) responsibilities:**
- Reviews each worker PR; integrates into main on merge
- Updates `docs/design/goat-client.md` and `docs/project/implementation-plan.md` Block 76 entry as workstreams land
- Keeps `goat-client/HANDOFF.md` (this file) fresh — strikes through completed tracks, adjusts estimates
- Notifies workers of cross-track interface changes (e.g., Track A's IPC method set changing affects Track B's GUI client)

**Worker responsibilities:**
- Read this HANDOFF + design doc + ADR before any code
- `/iso enter <track-name>` on their own VSCode session
- DO NOT touch other tracks' files
- DO NOT touch goat-trunk repo (this repo is goat-client; goat-trunk is separate)
- Commit small + push frequently for visibility; rebase on main before PR
- Sign commits with DCO sign-off (`-s`); GPG-sign per project convention; track trailer `[track: <name>]`
- Open PR when track's acceptance criterion is met; tag captain for review

**Pre-flight gate before any worker starts a track:**
1. `cd /Users/dene/src/github.com/dlf-dds/goat-client/`
2. `git fetch origin main && git pull --ff-only`
3. `/iso enter <track-name>` (provisions per-session worktree at `.claude/worktrees/<track-name>/`)
4. Read this HANDOFF + the relevant goat-trunk design doc
5. Start work. End-of-session: push branch, optionally open draft PR if not at acceptance yet.

---

## What's NOT yet decided

These are open questions the captain will resolve as work progresses; workers should flag them rather than guess:

- **Q1:** netbirdio/ios-client + netbirdio/android-client license — confirm Apache 2.0 (or whatever they are) before lifting Swift / Kotlin code from those repos. If incompatible: author from scratch using netbird's gomobile facade as the C-API contract.
- **Q2:** Mobile bundle-import UX — file-picker is straightforward; QR scan needs a QR-encoded bundle format spec (the CBOR bundle is ~1.5kB which fits comfortably in a QR-25 code; spec the encoding once during Track C/D rather than guessing).
- **Q3:** Auto-update — opt-in for v1 desktop per design doc Q3; what's the update channel (GitHub Releases? cosigned manifest?). Track E can leave a hook; v1 ships without auto-update.
- **Q4:** End-user probe-key delivery for snitch-app v2 — design doc Q3 says lean is bundle-extension; coordinate with snitch-app track when that activates.

---

## Scoring readiness for v1 desktop release

A worker should consider their track "done for v1 desktop" when:
- Track A: `go build ./...` green on 4 desktop targets + smoke-passes against a real wg-cp0 endpoint
- Track B: GUI launches on 3 desktop OSes; bundle import + connect + disconnect + tray indicator all work
- Track E: tagged release produces signed binaries; `cosign verify` passes
- Track F: install/uninstall works on at least one Linux distro + macOS + Windows; daemon auto-starts; uninstall is clean
- Track G: integration test green in CI; one real-protocol smoke against the lab green

v1 desktop release = all 5 of (A, B, E, F, G) green. Mobile (C + D) ships v1.5 / v2.

