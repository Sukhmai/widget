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
        // resource the worker has since fetched, or advance the story cycle.
        let next = Calendar.current.date(byAdding: .minute, value: 15, to: Date())!
        return Timeline(entries: [entry], policy: .after(next))
    }

    /// Resolve which scope to show (a pinned friend, or the merged "All"
    /// feed), fetch its active (last-24h) atoms, resolve the scope's story
    /// stack to pick which one to show, then fetch its poster.
    private func fetchEntry(for configuration: SelectFriendIntent) async -> StoryEntry {
        let scope = resolveScope(configuration)
        let client = DailyAPI.atomClient()

        var atoms: [Atom_V1_Atom] = []
        var avatarData: Data?

        switch scope {
        case .friend(let deviceID):
            guard let friend = FriendStore.friend(withID: deviceID) else {
                return StoryEntry(date: Date(), model: nil, posterData: nil, avatarData: nil)
            }
            avatarData = friend.avatar
            let response = await client.getActiveAtomsFromUser(
                request: .with { $0.userID = deviceID }
            )
            atoms = response.message?.atoms ?? []
        case .merged:
            let response = await client.getActiveFeedForSubscriber(
                request: .with { $0.subscriberID = DeviceUser.id }
            )
            atoms = response.message?.atoms ?? []
        }

        guard let topID = StoryProgress.resolveTop(freshAtomIDs: atoms.map(\.id), scope: scope),
              let atom = atoms.first(where: { $0.id == topID }) else {
            return StoryEntry(date: Date(), model: nil, posterData: nil, avatarData: avatarData)
        }

        if case .merged = scope {
            avatarData = FriendStore.friend(withID: atom.userID)?.avatar
        }

        let model = AtomCardModel(from: atom)
        if let linkURL = model.linkURL {
            StoryProgress.setTopURL(linkURL.absoluteString, for: scope)
        }
        var posterData: Data?
        if let url = model.coverURL {
            posterData = try? await URLSession.shared.data(from: url).0
        }
        return StoryEntry(date: Date(), model: model, posterData: posterData, avatarData: avatarData)
    }

    private func resolveScope(_ configuration: SelectFriendIntent) -> StoryScope {
        let selected = configuration.friend
        if let selected, selected.id != FriendSelection.randomID,
           FriendStore.friend(withID: selected.id) != nil {
            return .friend(selected.id)
        }
        return .merged
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
        // Downsampled decode, not UIImage(data:) — the widget extension has a
        // tight memory ceiling and full-resolution lazy decode risks jetsam.
        if let data, let ui = ImageDecoding.downsampledImage(from: data, maxDimension: 800) {
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
        .description("A friend's links, or everyone's, cycling through the last 24 hours.")
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
