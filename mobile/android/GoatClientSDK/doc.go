// Package goatclient is the gomobile-bound facade for the Android shell.
//
// On Android (//go:build android) it exports a Client struct + helper
// types (PlatformFiles, DNSList, EnvList, listeners, TunAdapter) consumed
// by the Kotlin VpnService shell at mobile/android/Shell/.
//
// On non-Android hosts the package is empty so `go build ./...` is green
// without an Android NDK / gomobile toolchain. The real artifact is built
// with:
//
//	gomobile bind -target=android \
//	  -javapkg=io.dlf_dds.goat_client.gomobile \
//	  -o mobile/android/Shell/app/libs/goat-client.aar \
//	  ./mobile/android/GoatClientSDK
//
// Forked from netbird `client/android/client.go` (BSD-3-Clause, see
// LICENSE.netbird-bsd3 + NOTICE) at upstream commit 3fc5a8d4a1fe.
// Heavily reshaped: Login / IsLoginRequired / SSO flows stripped; replaced
// with ImportBundle(bundleBytes []byte) error + GetTunnelStatus() string
// per design doc §wg-cp0-onboarding (offline-CA bundle, no auth flow).
//
// See mobile/android/README.md for the build pipeline.
package goatclient
