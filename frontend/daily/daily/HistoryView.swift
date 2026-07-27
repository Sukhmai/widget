//
//  HistoryView.swift
//  daily
//
//  History tab: a Friends/You picker up top. "You" drops straight into your
//  own post history; "Friends" lists friends, tapping one opens theirs.
//

import SwiftUI
import DailyKit

struct HistoryView: View {
    @State private var scope: Scope = .friends
    @State private var friends = FriendStore.load()

    private enum Scope: Hashable {
        case friends, you
    }

    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                Picker("Scope", selection: $scope) {
                    Text("Friends").tag(Scope.friends)
                    Text("You").tag(Scope.you)
                }
                .pickerStyle(.segmented)
                .padding()

                switch scope {
                case .friends: friendsList
                case .you: FriendHistoryView(userID: DeviceUser.id, title: "You")
                }
            }
            .navigationTitle("History")
            .navigationBarTitleDisplayMode(.inline)
        }
        .onAppear { friends = FriendStore.load() }
    }

    @ViewBuilder
    private var friendsList: some View {
        if friends.isEmpty {
            Spacer()
            Text("No friends yet").font(.footnote).foregroundStyle(Theme.textMuted)
            Spacer()
        } else {
            List(friends) { friend in
                NavigationLink(friend.name) {
                    FriendHistoryView(userID: friend.deviceID, title: friend.name)
                }
            }
            .listStyle(.plain)
        }
    }
}

#Preview {
    HistoryView()
}
