//
//  FriendsView.swift
//  daily
//
//  One-time own-number entry + contact matching. iOS can't read the device's
//  own number, so the user supplies it once; contacts are matched against the
//  backend and the resulting friends are stored for the widget to display.
//

import SwiftUI
import WidgetKit
import DailyKit

struct FriendsView: View {
    @State private var ownNumber = Social.ownNumber
    @State private var friends = FriendStore.load()
    @State private var syncing = false
    @State private var status: String?

    var body: some View {
        Form {
            Section("Your number") {
                TextField("e.g. +1 415 555 1234", text: $ownNumber)
                    .keyboardType(.phonePad)
                    .textContentType(.telephoneNumber)
                Text("Used so that your contacts can find you.")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            }

            Section {
                Button(action: { Task { await sync() } }) {
                    HStack {
                        if syncing { ProgressView().padding(.trailing, 4) }
                        Text(syncing ? "Matching…" : "Match contacts")
                    }
                }
                .disabled(syncing || ownNumber.trimmingCharacters(in: .whitespaces).isEmpty)
            } footer: {
                if let status { Text(status) }
            }

            if !friends.isEmpty {
                Section("Friends (\(friends.count))") {
                    ForEach(friends) { friend in
                        Text(friend.name)
                    }
                }
            }
        }
        .navigationTitle("Friends")
    }

    private func sync() async {
        syncing = true
        defer { syncing = false }
        status = nil

        // Persist + register the (possibly updated) own number first.
        Social.ownNumber = ownNumber.trimmingCharacters(in: .whitespacesAndNewlines)
        guard await Social.register() else {
            status = "Couldn't register that number — check the format."
            return
        }

        do {
            let contacts = try await ContactsImporter.fetch()
            friends = await Social.syncFriends(from: contacts)
            status = friends.isEmpty ? "No friends found yet." : "Found \(friends.count)."
            WidgetCenter.shared.reloadAllTimelines()
        } catch ContactsImporter.ImportError.accessDenied {
            status = "Contacts access denied. Enable it in Settings to match friends."
        } catch {
            status = "Something went wrong: \(error.localizedDescription)"
        }
    }
}

#Preview {
    NavigationStack { FriendsView() }
}
