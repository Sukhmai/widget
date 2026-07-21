//
//  ContentView.swift
//  daily
//
//  Created by Sukhmai Kapur on 7/20/26.
//

import SwiftUI
import WidgetKit
import DailyKit

struct ContentView: View {
    @Environment(\.openURL) private var openURL
    @State private var url = ""
    @State private var text = ""
    @State private var status: String?
    @State private var posting = false

    private var canPost: Bool {
        !posting && (!url.trimmingCharacters(in: .whitespaces).isEmpty
            || !text.trimmingCharacters(in: .whitespaces).isEmpty)
    }

    var body: some View {
        NavigationStack {
            Form {
                Section("Link") {
                    TextField("Paste a URL", text: $url)
                        .keyboardType(.URL)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                }
                Section("Speak on this") {
                    TextField("Only if you want", text: $text, axis: .vertical)
                        .lineLimit(3...6)
                }
                Section {
                    HStack {
                        Spacer()
                        Button(action: { Task { await post() } }) {
                            HStack {
                                if posting { ProgressView().padding(.trailing, 4) }
                                Text(posting ? "Posting…" : "Post")
                            }
                        }
                        .disabled(!canPost)
                        Spacer()
                    }
                    .listRowBackground(Color.clear)
                }
                if let status {
                    Section { Text(status).font(.footnote).foregroundStyle(.secondary) }
                }
            }
            .navigationTitle("Daily")
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    NavigationLink { FriendsView() } label: {
                        Image(systemName: "person.2")
                    }
                }
            }
        }
        // A widget tap always launches this app and delivers its `widgetURL` here
        // — iOS never opens Safari itself. Bounce web links back out to the browser
        // (or the app that owns the universal link).
        .onOpenURL { incoming in
            if incoming.scheme == "http" || incoming.scheme == "https" {
                openURL(incoming)
            }
        }
        // Re-register this device's discovery key on launch (no-op until the user
        // has entered their own number in FriendsView).
        .task {
            if Social.isRegistered { await Social.register() }
        }
    }

    private func post() async {
        posting = true
        defer { posting = false }
        status = nil

        let client = DailyAPI.atomClient()
        let response = await client.createAtom(request: .with {
            $0.userID = DeviceUser.id
            $0.text = text.trimmingCharacters(in: .whitespacesAndNewlines)
            $0.resourceURL = url.trimmingCharacters(in: .whitespacesAndNewlines)
        })

        if let error = response.error {
            status = "Failed: \(error.message ?? "unknown error")"
            return
        }

        url = ""
        text = ""
        status = "Posted!"
        WidgetCenter.shared.reloadAllTimelines()
    }
}

#Preview {
    ContentView()
}
