//
//  RootView.swift
//  daily
//
//  Bottom tab bar: History (friends' + your past posts) on the left, the
//  post composer centered and selected by default, Settings on the right.
//

import SwiftUI

struct RootView: View {
    @Environment(\.openURL) private var openURL
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
        // A widget tap always launches this app and delivers its `widgetURL`
        // here — iOS never opens Safari itself. Bounce web links back out to
        // the browser (or the app that owns the universal link); the
        // `daily://post` scheme is the widget's own compose shortcut, so it
        // just selects the Post tab instead.
        .onOpenURL { incoming in
            if incoming.scheme == "http" || incoming.scheme == "https" {
                openURL(incoming)
            } else if incoming.scheme == "daily", incoming.host == "post" {
                selection = .post
            }
        }
    }
}

#Preview {
    RootView()
}
