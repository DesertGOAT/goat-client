// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 dlf-dds contributors.

import SwiftUI

@main
struct GoatClientApp: App {
    @StateObject private var tunnel = TunnelManager()

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(tunnel)
                .task {
                    await tunnel.loadFromSystem()
                }
        }
    }
}
