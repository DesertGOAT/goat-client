// Package GoatClientSDK is the iOS-facing gomobile facade for goat-client.
//
// Built via `gomobile bind -target=ios` into a GoatClientSDK.xcframework that
// the Swift NEPacketTunnelProvider extension links against. The xcframework
// exposes a small Objective-C/Swift bridge: Client struct + ImportBundle / Run
// / Stop / GetTunnelStatus methods, plus a few helper types (EnvList,
// listener interfaces) implemented on the Swift side.
//
// Forked from netbird's client/ios/NetBirdSDK at upstream commit
// 3fc5a8d4a1fe308ff1068764a09b90b0859ab8fe (BSD-3-Clause; see LICENSE.netbird-bsd3
// + NOTICE at repo root). Heavy reshape: Login/OAuth methods replaced with
// ImportBundle (offline-CA-signed CBOR bundle); per-peer mesh status replaced
// with single-tunnel status (one wg-cp0 peer).
//
// Acceptance for Track C: this package builds with the iOS build tag set
// (`go build -tags=ios ./mobile/ios/GoatClientSDK/...` for type-checking on
// host), and `gomobile bind` (run via mobile/ios/scripts/build-xcframework.sh)
// produces a usable GoatClientSDK.xcframework for both iphoneos and
// iphonesimulator slices.
//
// Wiring to internal/bundle + internal/tunnel happens once Track A lands
// those packages; until then this facade dispatches to a stub backend that
// returns ErrTrackANotYetWired so the Swift shell can be developed and built
// against a stable API surface.
package GoatClientSDK
