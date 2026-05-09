// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 dlf-dds contributors.

import Foundation

/// BundleStore persists the imported CBOR bundle inside the App Group
/// container so the NEPacketTunnelProvider extension can read what the main
/// app imported. Both targets use this struct.
struct BundleStore {
    enum Error: Swift.Error {
        case noContainer
        case notFound
        case io(underlying: Swift.Error)
    }

    /// Persist `data` as the imported bundle. Overwrites any prior bundle.
    static func write(_ data: Data) throws {
        guard let container = AppGroup.containerURL else { throw Error.noContainer }
        let url = container.appendingPathComponent(AppGroup.bundleFileName)
        do {
            try data.write(to: url, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
        } catch {
            throw Error.io(underlying: error)
        }
    }

    /// Read the persisted bundle. Throws `.notFound` if no bundle has been
    /// imported yet.
    static func read() throws -> Data {
        guard let container = AppGroup.containerURL else { throw Error.noContainer }
        let url = container.appendingPathComponent(AppGroup.bundleFileName)
        guard FileManager.default.fileExists(atPath: url.path) else { throw Error.notFound }
        do {
            return try Data(contentsOf: url)
        } catch {
            throw Error.io(underlying: error)
        }
    }

    /// Whether a bundle has been imported.
    static var hasBundle: Bool {
        guard let container = AppGroup.containerURL else { return false }
        let url = container.appendingPathComponent(AppGroup.bundleFileName)
        return FileManager.default.fileExists(atPath: url.path)
    }

    /// Delete the persisted bundle. Idempotent.
    static func clear() {
        guard let container = AppGroup.containerURL else { return }
        let url = container.appendingPathComponent(AppGroup.bundleFileName)
        try? FileManager.default.removeItem(at: url)
    }

    /// Path the Go side passes as `cfgDir` to GoatClientSDK.NewClient.
    /// Equivalent to the App Group container path itself.
    static var cfgDir: String? {
        AppGroup.containerURL?.path
    }

    /// Path the Go side passes as `stateFile` — a JSON file inside the
    /// state/ subdir Track A's tunnel will read/write.
    static var stateFile: String? {
        guard let container = AppGroup.containerURL else { return nil }
        let dir = container.appendingPathComponent(AppGroup.stateDirName, isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir.appendingPathComponent("tunnel.json").path
    }
}
