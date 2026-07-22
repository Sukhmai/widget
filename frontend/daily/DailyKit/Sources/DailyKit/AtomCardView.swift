import SwiftUI
import WidgetKit

/// Ports take's `AtomCard.svelte` to the three iOS widget families:
///   systemSmall  → tile      (3:2 image on top, meta + text below — small square)
///   systemMedium → landscape (poster left, meta + text right)
///   systemLarge  → tile      (3:2 image on top, meta + text below — large square)
public struct AtomCardView: View {
    @Environment(\.widgetFamily) private var family
    private let model: AtomCardModel
    private let posterImage: Image?
    private let avatarImage: Image?

    public init(model: AtomCardModel, posterImage: Image?, avatarImage: Image? = nil) {
        self.model = model
        self.posterImage = posterImage
        self.avatarImage = avatarImage
    }

    public var body: some View {
        // The widget host supplies the background (`.containerBackground`) and the
        // system corner mask, so the card draws no shape or border of its own.
        // GeometryReader lets the poster size as a fraction of the container instead
        // of a fixed 120pt, so it fills each family proportionally.
        GeometryReader { proxy in
            content(in: proxy.size)
                // Fixed (not maxWidth/maxHeight: .infinity) so that if the content's
                // natural height overflows the container — e.g. fixedSize text on the
                // small widget — the reported frame stays true to the container and
                // the badge overlay below anchors to the real corner instead of the
                // overflowed one.
                .frame(width: proxy.size.width, height: proxy.size.height, alignment: .topLeading)
                .clipped()
                // The friend's contact avatar sits in the bottom-right corner of
                // the whole card, so it shows on every atom — poster or text-only.
                .overlay(alignment: .bottomTrailing) {
                    AvatarBadge(image: avatarImage, diameter: badgeDiameter(in: proxy.size))
                        .padding(Theme.space3)
                }
        }
        // A widget's default tap opens the container app; make the whole card
        // open the resource page instead — the native single-tap-target pattern.
        .widgetURL(model.linkURL)
    }

    /// Avatar size scales with the card but stays within a sensible badge range.
    private func badgeDiameter(in size: CGSize) -> CGFloat {
        min(max(min(size.width, size.height) * 0.2, 28), 40)
    }

    @ViewBuilder
    private func content(in size: CGSize) -> some View {
        switch family {
        case .systemSmall:  tile(in: size, textLineLimit: 2)
        case .systemLarge:  tile(in: size, textLineLimit: 4)
        default:            landscape(in: size)
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

    // ── tile (1×1 small / 2×2 large): full-width 3:2 hero image on top,
    // details + text below. Shared between both square families so the small
    // widget mirrors the large widget's layout, just scaled down.
    private func tile(in size: CGSize, textLineLimit: Int) -> some View {
        let imageHeight = (size.width - Theme.space3 * 2) * (2.0 / 3.0)
        return VStack(alignment: .leading, spacing: Theme.space3) {
            if model.hasCover {
                PosterView(image: posterImage)
                    .frame(maxWidth: .infinity)
                    .frame(height: imageHeight)
            }
            VStack(alignment: .leading, spacing: Theme.space2) {
                MetaView(model: model, showSubtitle: !model.subtitle.isEmpty)
                bodyOrUserText(lineLimit: textLineLimit)
            }
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
                    // Tinted home screens flatten widget content to a monochrome
                    // wash by default; the poster art is a photo, not chrome, so
                    // it should stay full color even in tinted mode.
                    image.resizable()
                        .widgetAccentedRenderingMode(.fullColor)
                        .scaledToFill()
                } else {
                    Theme.surface
                }
            }
            .clipShape(RoundedRectangle(cornerRadius: Theme.radiusSm))
    }
}

/// Circular contact avatar with a light ring + shadow so it reads on any poster.
/// Falls back to a neutral person glyph when the contact has no photo.
struct AvatarBadge: View {
    let image: Image?
    let diameter: CGFloat

    var body: some View {
        Group {
            if let image {
                // Same reasoning as the poster: a contact photo should render
                // full color rather than get tinted to the home screen's accent.
                image.resizable()
                    .widgetAccentedRenderingMode(.fullColor)
                    .scaledToFill()
            } else {
                Theme.surface.overlay {
                    Image(systemName: "person.fill")
                        .font(.system(size: diameter * 0.5))
                        .foregroundStyle(Theme.textMuted)
                }
            }
        }
        .frame(width: diameter, height: diameter)
        .clipShape(Circle())
        .overlay(Circle().strokeBorder(.white.opacity(0.9), lineWidth: 1.5))
        .shadow(color: .black.opacity(0.25), radius: 2, y: 1)
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
