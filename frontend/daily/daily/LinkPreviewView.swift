//
//  LinkPreviewView.swift
//  daily
//
//  Shown under the URL field as the user pastes a link: fetches display
//  metadata synchronously via `PreviewResource` and renders the same
//  `AtomCardView` the widget uses, sized to match each of the 3 widget
//  families so the user can see exactly what they're about to post.
//

import SwiftUI
import WidgetKit
import DailyKit

struct LinkPreviewView: View {
    let url: String
    let userText: String

    @State private var model: AtomCardModel?
    @State private var posterImage: Image?
    @State private var isLoading = false
    @State private var fetchFailed = false
    @State private var family: WidgetFamily = .systemMedium
    @State private var fetchTask: Task<Void, Never>?

    // Approximate iOS home screen widget point sizes (6.7" iPhone). Close
    // enough for a preview — exact size varies slightly by device.
    private var cardSize: CGSize {
        switch family {
        case .systemSmall: return CGSize(width: 170, height: 170)
        case .systemLarge: return CGSize(width: 364, height: 382)
        default:            return CGSize(width: 364, height: 170)
        }
    }

    // Widest card size (medium/large), used as a fixed picker width. The
    // picker must NOT size itself off `cardSize`, which is derived from
    // `family` — that would make selecting a tab resize the very control
    // mid-tap, fighting its own selection animation.
    private let pickerWidth: CGFloat = 364

    var body: some View {
        VStack(spacing: 12) {
            Picker("Widget size", selection: $family) {
                Text("Small").tag(WidgetFamily.systemSmall)
                Text("Medium").tag(WidgetFamily.systemMedium)
                Text("Large").tag(WidgetFamily.systemLarge)
            }
            .pickerStyle(.segmented)
            .frame(width: pickerWidth)
            // Guarantees the picker always wins hit-testing over the card
            // below it, regardless of the card's actual interactive bounds.
            .zIndex(1)

            ZStack {
                if let model {
                    AtomCardView(model: model, posterImage: posterImage, familyOverride: family)
                } else if fetchFailed {
                    Text("Couldn't generate a preview")
                        .font(.footnote)
                        .foregroundStyle(Theme.textMuted)
                }
            }
            .frame(width: cardSize.width, height: cardSize.height)
            .background(Theme.card)
            .clipShape(RoundedRectangle(cornerRadius: 22))
            .overlay {
                if isLoading {
                    ProgressView()
                }
            }
            // This is a static preview, never a tap target in-app (unlike the
            // real widget) — disabling hit-testing outright means it can never
            // steal touches from the picker above it, regardless of the exact
            // cause of its interactive bounds disagreeing with its visual ones.
            .allowsHitTesting(false)
        }
        .frame(maxWidth: .infinity)
        .animation(.default, value: model == nil)
        .onChange(of: url) { _, newValue in scheduleFetch(for: newValue) }
        .onChange(of: userText) { _, newValue in model?.userText = newValue }
        .onAppear { scheduleFetch(for: url) }
    }

    /// Debounces on the raw url text, then calls PreviewResource. Cancels any
    /// in-flight fetch so a fast typist never races an older response into
    /// view after a newer one.
    private func scheduleFetch(for rawURL: String) {
        fetchTask?.cancel()
        let trimmed = rawURL.trimmingCharacters(in: .whitespacesAndNewlines)
        guard let candidate = URL(string: trimmed), candidate.scheme != nil, candidate.host != nil else {
            model = nil
            isLoading = false
            fetchFailed = false
            return
        }

        fetchFailed = false
        let userText = self.userText
        fetchTask = Task {
            try? await Task.sleep(for: .milliseconds(500))
            guard !Task.isCancelled else { return }
            isLoading = true
            defer { isLoading = false }

            let response = await DailyAPI.atomClient().previewResource(request: .with { $0.url = trimmed })
            guard !Task.isCancelled else { return }

            guard response.error == nil, let resource = response.message?.resource else {
                model = nil
                fetchFailed = true
                return
            }

            let newModel = AtomCardModel(from: resource, userText: userText)
            var image: Image?
            if let coverURL = newModel.coverURL, let data = try? await URLSession.shared.data(from: coverURL).0 {
                #if canImport(UIKit)
                // Downsample off the main actor: the largest widget frame is
                // ~364pt wide, so 800px covers it at 2x with headroom.
                let ui = await Task.detached(priority: .userInitiated) {
                    ImageDecoding.downsampledImage(from: data, maxDimension: 800)
                }.value
                if let ui { image = Image(uiImage: ui) }
                #endif
            }
            guard !Task.isCancelled else { return }
            model = newModel
            posterImage = image
        }
    }
}

#Preview {
    LinkPreviewView(url: "https://letterboxd.com/film/marty-supreme/", userText: "one of the best of the year")
        .padding()
}
