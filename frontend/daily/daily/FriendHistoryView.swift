//
//  FriendHistoryView.swift
//  daily
//
//  A scrollable history of a friend's (or the device user's own) past posts,
//  reusing the same AtomCardView the widget and compose-time preview render.
//

import SwiftUI
import WidgetKit
import DailyKit

struct FriendHistoryView: View {
    let userID: String
    let title: String

    @State private var rows: [HistoryRow] = []
    @State private var hasNextPage = true
    @State private var isLoadingPage = false
    @State private var loadFailed = false
    @State private var family: WidgetFamily = .systemMedium
    @State private var avatarImage: Image?

    // Same approximate widget point sizes LinkPreviewView uses for its size
    // picker, so a friend's history reads exactly like the compose preview.
    private var cardSize: CGSize {
        switch family {
        case .systemSmall: return CGSize(width: 170, height: 170)
        case .systemLarge: return CGSize(width: 364, height: 382)
        default:            return CGSize(width: 364, height: 170)
        }
    }
    private let pickerWidth: CGFloat = 364

    // Small cards are half-width, so pack two per row; medium/large are
    // already full-width, so one per row.
    private var columns: [GridItem] {
        family == .systemSmall
            ? [GridItem(.fixed(cardSize.width), spacing: 12), GridItem(.fixed(cardSize.width), spacing: 12)]
            : [GridItem(.fixed(cardSize.width))]
    }

    var body: some View {
        ScrollView {
            VStack(spacing: 12) {
                Picker("Card size", selection: $family) {
                    Text("Small").tag(WidgetFamily.systemSmall)
                    Text("Medium").tag(WidgetFamily.systemMedium)
                    Text("Large").tag(WidgetFamily.systemLarge)
                }
                .pickerStyle(.segmented)
                .frame(width: pickerWidth)

                LazyVGrid(columns: columns, spacing: 12) {
                    ForEach(rows) { row in
                        card(for: row)
                            .frame(width: cardSize.width, height: cardSize.height)
                            .background(Theme.card)
                            .clipShape(RoundedRectangle(cornerRadius: 22))
                            .onAppear {
                                if row.id == rows.last?.id { Task { await loadNextPageIfNeeded() } }
                            }
                    }
                }

                if isLoadingPage {
                    ProgressView().padding()
                } else if rows.isEmpty && loadFailed {
                    Text("Couldn't load posts").font(.footnote).foregroundStyle(Theme.textMuted)
                } else if rows.isEmpty {
                    Text("No posts yet").font(.footnote).foregroundStyle(Theme.textMuted)
                }
            }
            .padding()
        }
        .background(Theme.surface)
        .navigationTitle(title)
        .task {
            loadAvatar()
            await loadNextPageIfNeeded()
        }
    }

    private func loadAvatar() {
        guard let data = FriendStore.friend(withID: userID)?.avatar else { return }
        #if canImport(UIKit)
        if let ui = ImageDecoding.downsampledImage(from: data, maxDimension: 800) {
            avatarImage = Image(uiImage: ui)
        }
        #endif
    }

    @ViewBuilder
    private func card(for row: HistoryRow) -> some View {
        let card = AtomCardView(model: row.model, posterImage: row.posterImage, avatarImage: avatarImage, familyOverride: family)
        if let linkURL = row.model.linkURL {
            Link(destination: linkURL) { card }
        } else {
            card
        }
    }

    private func loadNextPageIfNeeded() async {
        guard hasNextPage, !isLoadingPage else { return }
        isLoadingPage = true
        defer { isLoadingPage = false }

        let response = await DailyAPI.atomClient().listAtomsByUser(request: .with {
            $0.userID = userID
            if let last = rows.last {
                $0.cursorTime = last.createdAt
                $0.cursorID = last.id
            }
            $0.pageSize = 20
        })

        guard let message = response.message, response.error == nil else {
            loadFailed = true
            hasNextPage = false
            return
        }

        for atom in message.atoms {
            let model = AtomCardModel(from: atom)
            var posterImage: Image?
            if let coverURL = model.coverURL, let data = try? await URLSession.shared.data(from: coverURL).0 {
                #if canImport(UIKit)
                if let ui = ImageDecoding.downsampledImage(from: data, maxDimension: 800) {
                    posterImage = Image(uiImage: ui)
                }
                #endif
            }
            rows.append(HistoryRow(id: atom.id, createdAt: atom.createdAt, model: model, posterImage: posterImage))
        }
        hasNextPage = message.hasNextPage_p
    }
}

private struct HistoryRow: Identifiable {
    let id: String
    let createdAt: String
    let model: AtomCardModel
    let posterImage: Image?
}

#Preview {
    NavigationStack { FriendHistoryView(userID: "preview-user", title: "You") }
}
