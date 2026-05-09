// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 dlf-dds contributors.

import SwiftUI
import UniformTypeIdentifiers

struct ContentView: View {
    @EnvironmentObject private var tunnel: TunnelManager
    @State private var showingImporter = false
    @State private var importError: String?

    var body: some View {
        NavigationStack {
            VStack(spacing: 24) {
                statusCard
                bundleCard
                actionButtons
                Spacer()
            }
            .padding()
            .navigationTitle("goat-client")
            .fileImporter(
                isPresented: $showingImporter,
                allowedContentTypes: [.data, UTType(filenameExtension: "cbor") ?? .data],
                allowsMultipleSelection: false
            ) { result in
                handleImport(result)
            }
            .alert("Bundle import failed",
                   isPresented: Binding(get: { importError != nil }, set: { if !$0 { importError = nil } }),
                   actions: { Button("OK") { importError = nil } },
                   message: { Text(importError ?? "") })
        }
    }

    private var statusCard: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("Tunnel status")
                .font(.headline)
            HStack(spacing: 12) {
                Circle()
                    .fill(statusColor)
                    .frame(width: 12, height: 12)
                Text(tunnel.statusText)
                    .font(.body.monospaced())
            }
            if let last = tunnel.lastErrorText {
                Text(last)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(3)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding()
        .background(.thinMaterial, in: RoundedRectangle(cornerRadius: 12))
    }

    private var bundleCard: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("Onboarding bundle")
                .font(.headline)
            if BundleStore.hasBundle {
                Text("Bundle imported.")
                    .foregroundStyle(.secondary)
                Button("Replace bundle…") { showingImporter = true }
                Button("Clear bundle", role: .destructive) {
                    BundleStore.clear()
                    tunnel.refreshBundleState()
                }
            } else {
                Text("No bundle imported yet. Tap below to pick a `.cbor` bundle from Files / iCloud Drive / a sandbox lab share.")
                    .foregroundStyle(.secondary)
                Button("Import bundle…") { showingImporter = true }
                    .buttonStyle(.borderedProminent)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding()
        .background(.thinMaterial, in: RoundedRectangle(cornerRadius: 12))
    }

    private var actionButtons: some View {
        HStack(spacing: 12) {
            Button {
                Task { await tunnel.connect() }
            } label: {
                Label("Connect", systemImage: "play.circle.fill")
                    .frame(maxWidth: .infinity)
            }
            .buttonStyle(.borderedProminent)
            .disabled(!BundleStore.hasBundle || tunnel.status == .connecting || tunnel.status == .connected)

            Button {
                Task { await tunnel.disconnect() }
            } label: {
                Label("Disconnect", systemImage: "stop.circle.fill")
                    .frame(maxWidth: .infinity)
            }
            .buttonStyle(.bordered)
            .disabled(tunnel.status == .disconnected)
        }
    }

    private var statusColor: Color {
        switch tunnel.status {
        case .connected:    return .green
        case .connecting:   return .orange
        case .error:        return .red
        case .disconnected: return .gray
        }
    }

    private func handleImport(_ result: Result<[URL], Error>) {
        switch result {
        case .failure(let err):
            importError = err.localizedDescription
        case .success(let urls):
            guard let url = urls.first else { return }
            // The fileImporter hands us a security-scoped URL; we need to
            // start/stop the scope to read across the document-picker boundary.
            let scoped = url.startAccessingSecurityScopedResource()
            defer { if scoped { url.stopAccessingSecurityScopedResource() } }
            do {
                let data = try Data(contentsOf: url)
                guard data.count >= 64 else {
                    importError = "Selected file is too small (\(data.count) bytes); not a valid CBOR-signed bundle."
                    return
                }
                try BundleStore.write(data)
                tunnel.refreshBundleState()
            } catch {
                importError = error.localizedDescription
            }
        }
    }
}
