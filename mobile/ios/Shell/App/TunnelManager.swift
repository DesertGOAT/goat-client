// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 dlf-dds contributors.

import Foundation
import NetworkExtension
import Combine

/// TunnelManager wraps NETunnelProviderManager — the iOS-side handle to the
/// NEPacketTunnelProvider extension. Drives configuration save, start, stop,
/// and status polling for the SwiftUI surface.
@MainActor
final class TunnelManager: ObservableObject {
    enum Status {
        case disconnected, connecting, connected, error
    }

    /// The NE extension's bundle identifier — must match the `extension`
    /// target's bundle ID in project.yml.
    static let providerBundleID = "io.dlf-dds.goat-client.PacketTunnel"

    @Published var status: Status = .disconnected
    @Published var statusText: String = "disconnected"
    @Published var lastErrorText: String?

    private var manager: NETunnelProviderManager?
    private var statusObserver: NSObjectProtocol?

    deinit {
        if let token = statusObserver {
            NotificationCenter.default.removeObserver(token)
        }
    }

    /// Load the existing NE configuration (if any) from system settings, or
    /// create-and-save a fresh one. Idempotent — call on app launch.
    func loadFromSystem() async {
        do {
            let managers = try await NETunnelProviderManager.loadAllFromPreferences()
            if let existing = managers.first {
                self.manager = existing
            } else {
                self.manager = try await installFreshConfiguration()
            }
            attachStatusObserver()
            updateStatusFromConnection()
        } catch {
            self.lastErrorText = "Failed to load NE configuration: \(error.localizedDescription)"
            self.status = .error
            self.statusText = "error"
        }
    }

    /// Refresh published state when the bundle import state changes (e.g.
    /// after the user clears the bundle).
    func refreshBundleState() {
        objectWillChange.send()
    }

    func connect() async {
        guard let manager = manager else {
            lastErrorText = "Tunnel manager not loaded yet."
            return
        }
        guard BundleStore.hasBundle else {
            lastErrorText = "Import a bundle before connecting."
            return
        }
        do {
            // The NE extension reads the bundle from the App Group container,
            // so no per-call options are needed beyond a hint that the user
            // initiated this start (vs on-demand).
            try manager.connection.startVPNTunnel(options: ["userInitiated": NSNumber(value: true)])
            status = .connecting
            statusText = "connecting"
        } catch {
            lastErrorText = "startVPNTunnel: \(error.localizedDescription)"
            status = .error
            statusText = "error"
        }
    }

    func disconnect() async {
        guard let manager = manager else { return }
        manager.connection.stopVPNTunnel()
        status = .disconnected
        statusText = "disconnected"
    }

    // MARK: - private

    private func installFreshConfiguration() async throws -> NETunnelProviderManager {
        let manager = NETunnelProviderManager()
        let proto = NETunnelProviderProtocol()
        proto.providerBundleIdentifier = Self.providerBundleID
        // serverAddress is required by NETunnelProviderProtocol but isn't
        // meaningful for goat-client (the wg-cp0 endpoint comes from the
        // imported bundle, not from this field). Use a placeholder.
        proto.serverAddress = "goat-cp0"
        manager.protocolConfiguration = proto
        manager.localizedDescription = "goat-client"
        manager.isEnabled = true

        try await manager.saveToPreferences()
        // Re-load — `saveToPreferences` invalidates the in-memory connection
        // until you reload the manager from system preferences.
        try await manager.loadFromPreferences()
        return manager
    }

    private func attachStatusObserver() {
        guard let connection = manager?.connection else { return }
        if let token = statusObserver {
            NotificationCenter.default.removeObserver(token)
        }
        statusObserver = NotificationCenter.default.addObserver(
            forName: .NEVPNStatusDidChange,
            object: connection,
            queue: .main
        ) { [weak self] _ in
            // NotificationCenter callbacks are delivered on the main queue we
            // requested, so we hop to a Task for the @MainActor-isolated method.
            Task { @MainActor [weak self] in self?.updateStatusFromConnection() }
        }
    }

    private func updateStatusFromConnection() {
        guard let connection = manager?.connection else {
            status = .disconnected
            statusText = "disconnected"
            return
        }
        switch connection.status {
        case .invalid, .disconnected:
            status = .disconnected
            statusText = "disconnected"
        case .connecting, .reasserting:
            status = .connecting
            statusText = "connecting"
        case .connected:
            status = .connected
            statusText = "connected"
        case .disconnecting:
            status = .connecting
            statusText = "disconnecting"
        @unknown default:
            status = .error
            statusText = "unknown(\(connection.status.rawValue))"
        }
    }
}
