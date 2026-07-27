//
//  RootView.swift
//  daily
//
//  Bottom tab bar: History (friends' + your past posts) on the left, the
//  post composer centered and selected by default, Settings on the right.
//

import SwiftUI

struct RootView: View {
    @State private var selection: Tab = .post

    private enum Tab {
        case history, post, settings
    }

    var body: some View {
        TabView(selection: $selection) {
            HistoryView()
                .tabItem { Label("History", systemImage: "clock") }
                .tag(Tab.history)

            ContentView()
                .tabItem { Label("Post", systemImage: "plus.circle") }
                .tag(Tab.post)

            SettingsView()
                .tabItem { Label("Settings", systemImage: "gearshape") }
                .tag(Tab.settings)
        }
    }
}

#Preview {
    RootView()
}
