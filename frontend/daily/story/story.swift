//
//  story.swift
//  story
//
//  Created by Sukhmai Kapur on 7/20/26.
//

import WidgetKit
import SwiftUI
import DailyKit

struct Provider: TimelineProvider {
    func placeholder(in context: Context) -> StoryEntry {
        StoryEntry(date: Date(), model: .placeholder, posterData: nil)
    }

    func getSnapshot(in context: Context, completion: @escaping (StoryEntry) -> ()) {
        Task {
            completion(await fetchEntry())
        }
    }

    func getTimeline(in context: Context, completion: @escaping (Timeline<StoryEntry>) -> ()) {
        Task {
            let entry = await fetchEntry()
            // Refresh roughly every 15 minutes to pick up a newly posted atom or a
            // resource that has since been fetched by the worker.
            let next = Calendar.current.date(byAdding: .minute, value: 15, to: Date())!
            completion(Timeline(entries: [entry], policy: .after(next)))
        }
    }

    /// Pull the latest atom for this device and fetch its poster (if any).
    private func fetchEntry() async -> StoryEntry {
        let client = DailyAPI.atomClient()
        let response = await client.getLatestAtomFromUser(
            request: .with { $0.userID = DeviceUser.id }
        )

        guard let atom = response.message?.atom, response.message?.hasAtom == true else {
            return StoryEntry(date: Date(), model: nil, posterData: nil)
        }

        let model = AtomCardModel(from: atom)
        var posterData: Data?
        if let url = model.coverURL {
            posterData = try? await URLSession.shared.data(from: url).0
        }
        return StoryEntry(date: Date(), model: model, posterData: posterData)
    }
}

struct StoryEntry: TimelineEntry {
    let date: Date
    let model: AtomCardModel?
    let posterData: Data?
}

struct storyEntryView: View {
    var entry: Provider.Entry

    var body: some View {
        if let model = entry.model {
            AtomCardView(model: model, posterImage: posterImage)
        } else {
            emptyState
        }
    }

    private var posterImage: Image? {
        #if canImport(UIKit)
        if let data = entry.posterData, let ui = UIImage(data: data) {
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
        StaticConfiguration(kind: kind, provider: Provider()) { entry in
            storyEntryView(entry: entry)
                .containerBackground(Theme.card, for: .widget)
        }
        .configurationDisplayName("Daily")
        .description("Your friend's latest atom.")
        .supportedFamilies([.systemSmall, .systemMedium, .systemLarge])
        .contentMarginsDisabled()
    }
}

#Preview(as: .systemSmall) {
    story()
} timeline: {
    StoryEntry(date: .now, model: .placeholder, posterData: nil)
}

#Preview(as: .systemMedium) {
    story()
} timeline: {
    StoryEntry(date: .now, model: .placeholder, posterData: nil)
}

#Preview(as: .systemLarge) {
    story()
} timeline: {
    StoryEntry(date: .now, model: .placeholder, posterData: nil)
}
