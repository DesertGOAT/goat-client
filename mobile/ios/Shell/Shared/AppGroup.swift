// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 dlf-dds contributors.
//
// AppGroup defines the App Group container shared between the main app
// (which calls ImportBundle) and the NEPacketTunnelProvider extension
// (which reads the persisted bundle on startTunnel). Keep the identifier
// and the file names in lockstep across both targets.

import Foundation

enum AppGroup {
    /// App Group identifier. Must match `com.apple.security.application-groups`
    /// entitlement on both the main app target and the NE extension target.
    /// In the Apple Developer portal, App Groups for free-tier (no paid
    /// program) accounts are restricted to `group.<bundleID>` form, but the
    /// Simulator doesn't enforce that — any matching string works there.
    static let identifier = "group.io.dlf-dds.goat-client"

    /// URL of the App Group container. Returns nil only if the entitlement is
    /// misconfigured — call sites should treat that as a hard error.
    static var containerURL: URL? {
        FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: identifier)
    }

    /// File name (within the App Group container) where the imported CBOR
    /// bundle is persisted. The Go side (GoatClientSDK.ImportBundle) writes
    /// here; the NE extension's PacketTunnelProvider reads here on startTunnel.
    static let bundleFileName = "bundle.cbor"

    /// Subdirectory (within the App Group container) for the tunnel state
    /// JSON Track A's internal/tunnel will read/write (handshake counters,
    /// bytes in/out).
    static let stateDirName = "state"
}
