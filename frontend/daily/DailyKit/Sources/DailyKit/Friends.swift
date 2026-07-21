import Foundation

/// A matched friend: their device UUID (the write identity we read atoms by) and
/// the name from *this* device's contact book. The name never leaves the device
/// — the server only ever sees hashes and UUIDs.
public struct Friend: Codable, Hashable, Sendable, Identifiable {
    public var deviceID: String
    public var name: String
    /// Contact thumbnail (JPEG/PNG bytes) from this device's address book, shown
    /// as the avatar badge on the widget. Nil when the contact has no photo.
    public var avatar: Data?

    public var id: String { deviceID }

    public init(deviceID: String, name: String, avatar: Data? = nil) {
        self.deviceID = deviceID
        self.name = name
        self.avatar = avatar
    }
}

/// The on-device friend graph, persisted in the App Group so the `story` widget
/// can read what the app matched. Sorted by name for stable widget option order.
public enum FriendStore {
    private static let key = "DailyKit.friends"

    public static func load() -> [Friend] {
        guard let data = AppGroup.defaults.data(forKey: key),
              let friends = try? JSONDecoder().decode([Friend].self, from: data) else {
            return []
        }
        return friends
    }

    public static func save(_ friends: [Friend]) {
        let sorted = friends.sorted { $0.name.localizedCaseInsensitiveCompare($1.name) == .orderedAscending }
        guard let data = try? JSONEncoder().encode(sorted) else { return }
        AppGroup.defaults.set(data, forKey: key)
    }

    /// Look up a stored friend by device UUID (used by the widget to label the
    /// pinned selection).
    public static func friend(withID id: String) -> Friend? {
        load().first { $0.deviceID == id }
    }
}
