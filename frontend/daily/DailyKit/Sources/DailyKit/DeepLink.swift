import Foundation

/// Custom-scheme URLs the widget uses to jump straight into a specific app
/// screen, distinct from the http/https resource links atoms carry (which
/// bounce back out to Safari instead — see `RootView.onOpenURL`).
public enum DailyDeepLink {
    public static let post = URL(string: "daily://post")!
}
