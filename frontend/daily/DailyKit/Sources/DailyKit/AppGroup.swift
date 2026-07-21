import Foundation

/// Storage shared between the `daily` app and the `story` widget. The friend
/// list is built in the app (which alone can read Contacts) and read back by the
/// widget's timeline provider and configuration intent, so it must live in an
/// App Group — the one piece of state IDFV can't cover.
///
/// Both targets must declare `com.apple.security.application-groups` containing
/// `AppGroup.identifier` in their entitlements, or `defaults` falls back to nil.
public enum AppGroup {
    public static let identifier = "group.skapur.sharedaily"

    /// Shared defaults, or `.standard` if the App Group isn't configured yet
    /// (keeps previews/tests from crashing; real cross-process sharing needs it).
    public static var defaults: UserDefaults {
        UserDefaults(suiteName: identifier) ?? .standard
    }
}
