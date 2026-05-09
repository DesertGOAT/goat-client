// Package main is the goat-client daemon binary.
//
// Drives wireguard-go (or kernel WG on Linux) for the wg-cp0 outer tunnel.
// Consumes the offline-CA-signed CBOR bundle for onboarding.
//
// On desktop: runs as a system service (systemd / launchd / Windows Service).
// On mobile: linked into the Swift NEPacketTunnelProvider (iOS) or Kotlin
// VpnService (Android) via gomobile bindings (see mobile/ios + mobile/android).
package main

import "fmt"

func main() {
	fmt.Println("goat-clientd: scaffolding stub — see HANDOFF.md for parallel build-out tracks")
}
