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
    private enum Field: Hashable { case url, text }
    @FocusState private var focusedField: Field?

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
                        .focused($focusedField, equals: .url)
                        // A paste lands the whole URL in one change (many
                        // characters at once); typing adds one at a time. Only
                        // dismiss on the former, so the preview has room to show
                        // without interrupting someone still typing a URL by hand.
                        .onChange(of: url) { old, new in
                            if new.count > old.count + 1 {
                                focusedField = nil
                            }
                        }
                }
                Section("Speak on this") {
                    TextField("Only if you want", text: $text, axis: .vertical)
                        .lineLimit(3...6)
                        .focused($focusedField, equals: .text)
                }
            }
            // Form/List has no tap-outside-to-dismiss by default — only
            // interactive scrolling and focus changes dismiss the keyboard for
            // free. A plain (non-simultaneous) tap gesture on the Form itself
            // only fires for taps that land on blank row background, since a
            // descendant control's own gesture (TextField, Button) still wins
            // for taps on it — so this doesn't interfere with either field.
            .onTapGesture { focusedField = nil }
            // The preview's card resizes per widget-size tab, and one of its
            // sizes (Large) hosts an Image behind a GeometryReader. Inside a
            // Form/List row, UIKit's self-sizing-cell solver can pathologically
            // stall resolving that combination's height — moving it to a
            // safeAreaInset takes it out of the List entirely, so it's a plain
            // resize instead of a table cell re-layout.
            .safeAreaInset(edge: .bottom) {
                VStack(spacing: 12) {
                    if !url.trimmingCharacters(in: .whitespaces).isEmpty {
                        LinkPreviewView(url: url, userText: text)
                    }
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
                    if let status {
                        Text(status).font(.footnote).foregroundStyle(.secondary)
                    }
                }
                .padding()
                .background(.bar)
                .onTapGesture { focusedField = nil }
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
            await syncFriendsIfDue()
        }
    }

    /// Silently re-reads Contacts and re-syncs friends on a throttled cadence.
    /// New signups are already caught server-side (MatchContacts remembers each
    /// subscriber's contact hashes and auto-subscribes them when a match later
    /// registers) — this instead catches what only a fresh Contacts read can:
    /// contacts added, edited, or removed on-device. Reading Contacts again
    /// after the first grant never re-prompts, so this can run unattended.
    private func syncFriendsIfDue() async {
        let resyncInterval: TimeInterval = 7 * 24 * 3600
        guard Social.isRegistered, ContactsImporter.isAuthorized else { return }
        if let last = Social.lastSyncedAt, Date().timeIntervalSince(last) < resyncInterval { return }

        guard let contacts = try? await ContactsImporter.fetch() else { return }
        await Social.syncFriends(from: contacts)
        WidgetCenter.shared.reloadAllTimelines()
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
