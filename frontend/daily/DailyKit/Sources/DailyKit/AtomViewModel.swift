import Foundation

/// Which brand a resource came from — drives the corner logo. Only the sources
/// we render a logo for need a case; everything else uses `.other`.
public enum ResourceSource: String, Sendable {
    case letterboxd
    case other

    public var logoAssetName: String? {
        switch self {
        case .letterboxd: return "letterboxd"
        case .other: return nil
        }
    }
}

/// Render layout for a resource, mirroring take's `ResourceSublayout`.
///   media — cover image + title + subtitle
///   text  — no cover; title/subtitle/body shown as text (tweet/article/etc.)
public enum ResourceSublayout: Sendable {
    case media
    case text
}

/// SwiftUI analogue of take's `AtomView` (`resources/atom.ts`). Flattens an
/// `Atom_V1_Atom` into the fields the card renders.
public struct AtomCardModel: Sendable {
    public var source: ResourceSource
    public var sublayout: ResourceSublayout
    public var coverURL: URL?
    public var linkURL: URL?       // canonical page for the resource (tap target)
    public var title: String
    public var subtitle: String
    public var bodyText: String   // resource body (tweet/article), if any
    public var userText: String   // the author's own text on the atom

    public init(
        source: ResourceSource,
        sublayout: ResourceSublayout,
        coverURL: URL?,
        linkURL: URL? = nil,
        title: String,
        subtitle: String,
        bodyText: String,
        userText: String
    ) {
        self.source = source
        self.sublayout = sublayout
        self.coverURL = coverURL
        self.linkURL = linkURL
        self.title = title
        self.subtitle = subtitle
        self.bodyText = bodyText
        self.userText = userText
    }

    /// Whether the model shows a cover image (media sublayout with a URL).
    public var hasCover: Bool { sublayout == .media && coverURL != nil }

    /// Sample model for previews/placeholders (Marty Supreme).
    public static let placeholder = AtomCardModel(
        source: .letterboxd,
        sublayout: .media,
        coverURL: nil,
        linkURL: URL(string: "https://letterboxd.com/film/marty-supreme/"),
        title: "Marty Supreme",
        subtitle: "2025 · Josh Safdie",
        bodyText: "",
        userText: "one of the best of the year"
    )
}

private func joined(_ parts: String?...) -> String {
    parts.compactMap { $0 }.filter { !$0.isEmpty }.joined(separator: " · ")
}

private func year(_ y: Int32) -> String? { y > 0 ? String(y) : nil }

public extension AtomCardModel {
    /// Resolve an atom into a card model, mirroring take's `resolveAtomView`.
    init(from atom: Atom_V1_Atom) {
        self.init(from: atom.hasResource ? atom.resource : Atom_V1_Resource(), userText: atom.text)
    }

    /// Resolve a bare resource — e.g. the live result of `PreviewResource`,
    /// not yet attached to any atom — into a card model.
    init(from resource: Atom_V1_Resource, userText: String) {
        func url(_ s: String) -> URL? { s.isEmpty ? nil : URL(string: s) }

        // Canonical page for the resource — the tap target for the widget link.
        let link = url(resource.url)

        guard resource.fetched, let meta = resource.metadata else {
            self.init(source: .other, sublayout: .text, coverURL: nil, linkURL: link,
                      title: "", subtitle: "", bodyText: "", userText: userText)
            return
        }

        switch meta {
        case .movie(let m), .imdbMovie(let m):
            self.init(source: .letterboxd, sublayout: .media, coverURL: url(m.posterURL),
                      title: m.title, subtitle: joined(year(m.releaseYear), m.director),
                      bodyText: "", userText: userText)
        case .spotifyTrack(let t):
            self.init(source: .other, sublayout: .media, coverURL: url(t.coverURL),
                      title: t.title, subtitle: t.artist, bodyText: "", userText: userText)
        case .book(let b):
            self.init(source: .other, sublayout: .media, coverURL: url(b.coverURL),
                      title: b.title, subtitle: joined(year(b.publicationYear), b.author),
                      bodyText: "", userText: userText)
        case .steamGame(let g):
            self.init(source: .other, sublayout: .media, coverURL: url(g.coverURL),
                      title: g.title, subtitle: g.developer, bodyText: "", userText: userText)
        case .youtubeVideo(let y):
            self.init(source: .other, sublayout: .media, coverURL: url(y.coverURL),
                      title: y.title, subtitle: y.channelName, bodyText: "", userText: userText)
        case .tiktokVideo(let t):
            self.init(source: .other, sublayout: .media, coverURL: url(t.coverURL),
                      title: t.title, subtitle: t.creator, bodyText: "", userText: userText)
        case .malAnime(let a), .anilistAnime(let a):
            self.init(source: .other, sublayout: .media, coverURL: url(a.coverURL),
                      title: a.title, subtitle: joined(year(a.releaseYear), a.studio),
                      bodyText: "", userText: userText)
        case .instagramPost(let i):
            self.init(source: .other, sublayout: .media, coverURL: url(i.coverURL),
                      title: i.authorName.isEmpty ? "@\(i.handle)" : i.authorName,
                      subtitle: "@\(i.handle)", bodyText: "", userText: userText)
        case .tweet(let t):
            self.init(source: .other, sublayout: .text, coverURL: nil,
                      title: "@\(t.handle)", subtitle: t.authorName,
                      bodyText: t.text, userText: userText)
        case .blueskyPost(let b):
            self.init(source: .other, sublayout: .text, coverURL: nil,
                      title: "@\(b.handle)", subtitle: b.authorName,
                      bodyText: b.text, userText: userText)
        case .wikipedia(let w):
            self.init(source: .other, sublayout: .text, coverURL: url(w.thumbnailURL),
                      title: w.title, subtitle: w.description_p,
                      bodyText: w.extract, userText: userText)
        case .substackPost(let s):
            self.init(source: .other, sublayout: .media, coverURL: url(s.coverURL),
                      title: s.title, subtitle: s.authorName, bodyText: "", userText: userText)
        case .generic(let g):
            // generic is the only source whose sublayout is data-driven: an
            // article renders like wikipedia/tweet (text sublayout, body copy),
            // anything else renders like a media card (poster + title/site).
            // Mirrors take's resolveAtomView (frontend/src/lib/resources/atom.ts).
            if g.isArticle {
                self.init(source: .other, sublayout: .text,
                          coverURL: url(g.imageURL),
                          title: g.title.isEmpty ? g.siteName : g.title,
                          subtitle: g.author.isEmpty ? g.siteName : g.author,
                          bodyText: g.bodyText.isEmpty ? g.description_p : g.bodyText,
                          userText: userText)
            } else {
                // Favicon falls back as a media thumbnail — a blown-up favicon
                // only looks bad full-width for the article hero, which this
                // branch never renders.
                let coverURLString = g.imageURL.isEmpty ? g.faviconURL : g.imageURL
                self.init(source: .other, sublayout: .media,
                          coverURL: url(coverURLString),
                          title: g.title.isEmpty ? g.siteName : g.title,
                          subtitle: g.siteName, bodyText: "", userText: userText)
            }
        case .image(let i):
            self.init(source: .other, sublayout: .media, coverURL: url(i.imageURL),
                      title: i.siteName, subtitle: "", bodyText: "", userText: userText)
        case .redditPost(let r):
            self.init(source: .other, sublayout: .text, coverURL: nil,
                      title: r.title, subtitle: joined("u/\(r.author)", "r/\(r.subreddit)"),
                      bodyText: r.bodyText, userText: userText)
        }
        self.linkURL = link
    }
}
