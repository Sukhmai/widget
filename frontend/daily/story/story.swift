//
//  story.swift
//  story
//
//  Created by Sukhmai Kapur on 7/20/26.
//

import WidgetKit
import SwiftUI
import DailyKit

struct Provider: AppIntentTimelineProvider {
    func placeholder(in context: Context) -> StoryEntry {
        StoryEntry(date: Date(), model: .placeholder, posterData: nil, avatarData: nil)
    }

    func snapshot(for configuration: SelectFriendIntent, in context: Context) async -> StoryEntry {
        await fetchEntry(for: configuration)
    }

    func timeline(for configuration: SelectFriendIntent, in context: Context) async -> Timeline<StoryEntry> {
        let entry = await fetchEntry(for: configuration)
        // Refresh roughly every 15 minutes to pick up a newly posted atom, a
        // resource the worker has since fetched, or (in Random mode) a new friend.
        let next = Calendar.current.date(byAdding: .minute, value: 15, to: Date())!
        return Timeline(entries: [entry], policy: .after(next))
    }

    /// Resolve which friend's atom to show, then fetch it and its poster.
    /// A pinned friend uses their device_id; Random (or an unset/removed
    /// selection) picks a matched friend at random each refresh.
    private func fetchEntry(for configuration: SelectFriendIntent) async -> StoryEntry {
        guard let friend = resolveFriend(configuration) else {
            return StoryEntry(date: Date(), model: nil, posterData: nil, avatarData: nil)
        }

        let client = DailyAPI.atomClient()
        let response = await client.getLatestAtomFromUser(
            request: .with { $0.userID = friend.deviceID }
        )

        guard let atom = response.message?.atom, response.message?.hasAtom == true else {
            return StoryEntry(date: Date(), model: nil, posterData: nil, avatarData: friend.avatar)
        }

        let model = AtomCardModel(from: atom)
        var posterData: Data?
        if let url = model.coverURL {
            posterData = try? await URLSession.shared.data(from: url).0
        }
        return StoryEntry(date: Date(), model: model, posterData: posterData, avatarData: friend.avatar)
    }

    private func resolveFriend(_ configuration: SelectFriendIntent) -> Friend? {
        let selected = configuration.friend
        if let selected, selected.id != FriendSelection.randomID {
            // Pinned friend — honor it only if still in the matched list.
            return FriendStore.friend(withID: selected.id)
        }
        // Random / unset: pick from the matched friends.
        return FriendStore.load().randomElement()
    }
}

struct StoryEntry: TimelineEntry {
    let date: Date
    let model: AtomCardModel?
    let posterData: Data?
    let avatarData: Data?
}

struct storyEntryView: View {
    var entry: Provider.Entry

    var body: some View {
        if let model = entry.model {
            AtomCardView(model: model, posterImage: posterImage, avatarImage: avatarImage)
        } else {
            emptyState
        }
    }

    private var posterImage: Image? { image(from: entry.posterData) }
    private var avatarImage: Image? { image(from: entry.avatarData) }

    private func image(from data: Data?) -> Image? {
        #if canImport(UIKit)
        if let data, let ui = UIImage(data: data) {
            return Image(uiImage: ui)
        }
        #endif
        return nil
    }

    private var emptyState: some View {
        Text("No atom yet")
            .font(.footnote)
            .foregroundStyle(Theme.textMuted)
            .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

struct story: Widget {
    let kind: String = "story"

    var body: some WidgetConfiguration {
        AppIntentConfiguration(kind: kind, intent: SelectFriendIntent.self, provider: Provider()) { entry in
            storyEntryView(entry: entry)
                .containerBackground(Theme.card, for: .widget)
        }
        .configurationDisplayName("Daily")
        .description("A friend's latest atom. Long-press to pick a friend or rotate at random.")
        .supportedFamilies([.systemSmall, .systemMedium, .systemLarge])
        .contentMarginsDisabled()
    }
}

#Preview(as: .systemSmall) {
    story()
} timeline: {
    StoryEntry(date: .now, model: .placeholder, posterData: nil, avatarData: nil)
}

#Preview(as: .systemMedium) {
    story()
} timeline: {
    StoryEntry(date: .now, model: .placeholder, posterData: nil, avatarData: nil)
}

#Preview(as: .systemLarge) {
    story()
} timeline: {
    StoryEntry(date: .now, model: .placeholder, posterData: nil, avatarData: nil)
}
