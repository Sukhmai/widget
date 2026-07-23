import SwiftUI
import WidgetKit

/// Ports take's `AtomCard.svelte` to the three iOS widget families:
///   systemSmall  → tile      (3:2 image on top, meta + text below — small square)
///   systemMedium → landscape (poster left, meta + text right)
///   systemLarge  → tile      (3:2 image on top, meta + text below — large square)
public struct AtomCardView: View {
    @Environment(\.widgetFamily) private var environmentFamily
    private let model: AtomCardModel
    private let posterImage: Image?
    private let avatarImage: Image?
    private let familyOverride: WidgetFamily?

    /// `familyOverride` lets a plain (non-widget) SwiftUI view render this
    /// card as a specific family — e.g. an in-app size preview — without
    /// WidgetKit's `\.widgetFamily` environment value, which isn't writable
    /// outside an actual widget extension context.
    public init(model: AtomCardModel, posterImage: Image?, avatarImage: Image? = nil, familyOverride: WidgetFamily? = nil) {
        self.model = model
        self.posterImage = posterImage
        self.avatarImage = avatarImage
        self.familyOverride = familyOverride
    }

    private var family: WidgetFamily { familyOverride ?? environmentFamily }

    // `.widgetAccentedRenderingMode` negotiates tinted-home-screen rendering
    // with the widget host; it's only meaningful inside an actual widget
    // extension. `familyOverride` is only ever set by a plain in-app preview
    // (see the doc comment on `init` above), so its presence also tells us
    // there's no real widget host to negotiate with.
    private var isWidgetContext: Bool { familyOverride == nil }

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
                    AvatarBadge(image: avatarImage, diameter: badgeDiameter(in: proxy.size), isWidgetContext: isWidgetContext)
                        .padding(Theme.space3)
                }
        }
        // A widget's default tap opens the container app; make the whole card
        // open the resource page instead — the native single-tap-target pattern.
        // Widget-host-only, like widgetAccentedRenderingMode below: outside an
        // actual widget extension there's no host to consume this, and passing
        // a real URL here caused it to intercept taps meant for sibling controls.
        .widgetURL(isWidgetContext ? model.linkURL : nil)
    }

    /// Avatar size scales with the card but stays within a sensible badge range.
    private func badgeDiameter(in size: CGSize) -> CGFloat {
        min(max(min(size.width, size.height) * 0.2, 28), 40)
    }

    @ViewBuilder
    private func content(in size: CGSize) -> some View {
        switch family {
        // The small widget has no room for both a cover image and body text
        // legibly, so text only shows when there's no image taking up space
        // (e.g. articles/tweets with no poster).
        case .systemSmall:  tile(in: size, textLineLimit: model.hasCover ? nil : 3)
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
                PosterView(image: posterImage, isWidgetContext: isWidgetContext).frame(width: side, height: side)
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
    private func tile(in size: CGSize, textLineLimit: Int?) -> some View {
        let imageHeight = (size.width - Theme.space3 * 2) * (2.0 / 3.0)
        return VStack(alignment: .leading, spacing: Theme.space3) {
            if model.hasCover {
                PosterView(image: posterImage, isWidgetContext: isWidgetContext)
                    .frame(maxWidth: .infinity)
                    .frame(height: imageHeight)
            }
            VStack(alignment: .leading, spacing: Theme.space2) {
                MetaView(model: model, showSubtitle: !model.subtitle.isEmpty)
                if let textLineLimit {
                    bodyOrUserText(lineLimit: textLineLimit)
                }
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
    var isWidgetContext: Bool = true

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
                    // it should stay full color even in tinted mode. Only meaningful
                    // (and safe) inside an actual widget host — see isWidgetContext.
                    if isWidgetContext {
                        image.resizable()
                            .widgetAccentedRenderingMode(.fullColor)
                            .scaledToFill()
                    } else {
                        image.resizable()
                            .scaledToFill()
                    }
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
    var isWidgetContext: Bool = true

    var body: some View {
        Group {
            if let image {
                // Same reasoning as the poster: a contact photo should render
                // full color rather than get tinted to the home screen's accent.
                // Only meaningful (and safe) inside an actual widget host.
                if isWidgetContext {
                    image.resizable()
                        .widgetAccentedRenderingMode(.fullColor)
                        .scaledToFill()
                } else {
                    image.resizable()
                        .scaledToFill()
                }
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
