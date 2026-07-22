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
            handleWidgetTap(url: incoming.absoluteString)
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

    /// A story widget tap always lands here via onOpenURL before the URL
    /// bounces out. The tapped URL is matched back against each scope's
    /// top-of-stack URL (recorded by story.swift's Provider) to find which
    /// scope(s) it came from — usually exactly one, but resources are
    /// deduplicated by URL server-side, so two friends sharing the same link
    /// can both match; we mark/pop both rather than guessing.
    ///
    /// `reloadTimelines(ofKind:)` re-renders *every* story widget instance,
    /// including ones showing unrelated scopes — WidgetKit has no API to
    /// target a single configured instance. Marking viewed is already scoped
    /// to the matched scope(s) via the URL match above; `requestPop` scopes
    /// the pop the same way, so untapped instances that get swept up in the
    /// reload just re-render their current top instead of popping early
    /// (see StoryProgress.resolveTop).
    private func handleWidgetTap(url: String) {
        let scopes = StoryProgress.scopesShowing(url: url)
        for scope in scopes {
            if let atomID = StoryProgress.top(for: scope) {
                StoryProgress.markViewed(atomID)
            }
            StoryProgress.requestPop(for: scope)
        }
        guard !scopes.isEmpty else { return }
        WidgetCenter.shared.reloadTimelines(ofKind: "story")
    }
}

#Preview {
    ContentView()
}
