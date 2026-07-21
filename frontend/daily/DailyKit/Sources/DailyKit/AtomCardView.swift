import SwiftUI
import WidgetKit

/// Ports take's `AtomCard.svelte` to the three iOS widget families:
///   systemSmall  → compact   (full-bleed poster)
///   systemMedium → landscape (poster left, meta + text right)
///   systemLarge  → tile      (poster + meta on top, text below)
public struct AtomCardView: View {
    @Environment(\.widgetFamily) private var family
    private let model: AtomCardModel
    private let posterImage: Image?

    public init(model: AtomCardModel, posterImage: Image?) {
        self.model = model
        self.posterImage = posterImage
    }

    public var body: some View {
        // The widget host supplies the background (`.containerBackground`) and the
        // system corner mask, so the card draws no shape or border of its own.
        // GeometryReader lets the poster size as a fraction of the container instead
        // of a fixed 120pt, so it fills each family proportionally.
        GeometryReader { proxy in
            content(in: proxy.size)
                .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        }
        // A widget's default tap opens the container app; make the whole card
        // open the resource page instead — the native single-tap-target pattern.
        .widgetURL(model.linkURL)
    }

    @ViewBuilder
    private func content(in size: CGSize) -> some View {
        switch family {
        case .systemSmall:  compact
        case .systemLarge:  tile(in: size)
        default:            landscape(in: size)
        }
    }

    // ── compact (1×1): full-bleed poster, or text-only fallback ──
    @ViewBuilder
    private var compact: some View {
        if model.hasCover {
            PosterView(image: posterImage)
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .clipped()
        } else {
            VStack(alignment: .leading, spacing: Theme.space2) {
                MetaView(model: model, showSubtitle: !model.subtitle.isEmpty)
                bodyOrUserText(lineLimit: 6)
                Spacer(minLength: 0)
            }
            .padding(Theme.space3)
        }
    }

    // ── landscape (2×1): poster left, meta + text right ──
    // Poster is a square as tall as the padded content, capped at half the width.
    private func landscape(in size: CGSize) -> some View {
        let side = min(size.height - Theme.space3 * 2, size.width * 0.5)
        return HStack(alignment: .top, spacing: Theme.space3) {
            if model.hasCover {
                PosterView(image: posterImage).frame(width: side, height: side)
            }
            VStack(alignment: .leading, spacing: Theme.space2) {
                MetaView(model: model, showSubtitle: !model.subtitle.isEmpty)
                bodyOrUserText(lineLimit: 4)
                Spacer(minLength: 0)
            }
        }
        .padding(Theme.space3)
    }

    // ── tile (2×2): poster + meta on top, text below ──
    // Poster fills ~42% of the tile width so the top row reads as a proper hero row
    // rather than a small thumbnail lost in whitespace.
    private func tile(in size: CGSize) -> some View {
        let side = min(size.width * 0.42, (size.height - Theme.space3 * 2) * 0.55)
        return VStack(alignment: .leading, spacing: Theme.space3) {
            HStack(alignment: .top, spacing: Theme.space3) {
                if model.hasCover {
                    PosterView(image: posterImage).frame(width: side, height: side)
                }
                MetaView(model: model, showSubtitle: !model.subtitle.isEmpty)
                Spacer(minLength: 0)
            }
            bodyOrUserText(lineLimit: 8)
            Spacer(minLength: 0)
        }
        .padding(Theme.space3)
    }

    /// The resource body (tweet/article) takes priority; otherwise the author's
    /// own text. Empty → nothing.
    @ViewBuilder
    private func bodyOrUserText(lineLimit: Int) -> some View {
        let text = !model.bodyText.isEmpty ? model.bodyText : model.userText
        if !text.isEmpty {
            Text(text)
                .font(.footnote)
                .foregroundStyle(Theme.text)
                .lineLimit(lineLimit)
                .fixedSize(horizontal: false, vertical: true)
        }
    }

}

/// Rounded cover image with a gray placeholder while the poster is unavailable.
struct PosterView: View {
    let image: Image?

    var body: some View {
        // `Color.clear` adopts the caller's proposed size (120×120 in the tile /
        // landscape, or the full slot in compact); the poster rides in an overlay
        // that inherits those bounds. `scaledToFill` still overflows, but the
        // composed view's layout size stays the container's, so `clipShape` crops
        // the overflow instead of letting it bleed past the poster's slot.
        Color.clear
            .overlay {
                if let image {
                    image.resizable().scaledToFill()
                } else {
                    Theme.surface
                }
            }
            .clipShape(RoundedRectangle(cornerRadius: Theme.radiusSm))
    }
}

/// Title (13pt medium) + optional subtitle (12pt muted).
struct MetaView: View {
    let model: AtomCardModel
    let showSubtitle: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            if !model.title.isEmpty {
                Text(model.title)
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(Theme.text)
                    .lineLimit(1)
            }
            if showSubtitle {
                Text(model.subtitle)
                    .font(.caption)
                    .foregroundStyle(Theme.textMuted)
                    .lineLimit(1)
            }
        }
    }
}
