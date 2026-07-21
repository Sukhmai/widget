import SwiftUI

/// Design tokens. Colors are fully adaptive — they resolve against the viewer's
/// light/dark environment and home-screen tint rather than the fixed light-web
/// values the take webapp used. Typography lives at the call sites as semantic
/// text styles (`.headline`, `.subheadline`, …) so it scales with Dynamic Type.
public enum Theme {
    // Colors (adaptive)
    public static let text: Color = .primary       // titles, body
    public static let textMuted: Color = .secondary // meta / subtitles

    #if canImport(UIKit)
    /// Widget container surface — matches the system's grouped background.
    public static let card = Color(.systemBackground)
    /// Poster placeholder while the image is unavailable.
    public static let surface = Color(.secondarySystemBackground)
    #else
    public static let card = Color.white
    public static let surface = Color.gray.opacity(0.15)
    #endif

    // Radii (px → pt)
    public static let radiusSm: CGFloat = 8
    public static let radiusMd: CGFloat = 14

    // Spacing
    public static let space2: CGFloat = 8
    public static let space3: CGFloat = 12
}
