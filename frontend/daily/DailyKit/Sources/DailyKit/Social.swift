import Foundation

/// Registration + contact-matching orchestration, shared so the app owns the
/// Contacts read (permission-gated, app-only) while the hashing, RPCs, and store
/// writes live here in the kit.
public enum Social {
    private static let ownNumberKey = "DailyKit.ownNumber"

    /// The user's own number, entered once (iOS can't read it for us). Persisted
    /// in the App Group; empty until set.
    public static var ownNumber: String {
        get { AppGroup.defaults.string(forKey: ownNumberKey) ?? "" }
        set { AppGroup.defaults.set(newValue, forKey: ownNumberKey) }
    }

    public static var isRegistered: Bool { !ownNumber.isEmpty }

    /// Register (or re-point) this device's discovery key from the stored own
    /// number. No-op if the number is unset or unparseable. Safe to call on every
    /// launch — the backend upserts by device_id.
    @discardableResult
    public static func register() async -> Bool {
        guard let phoneHash = PhoneHash.hash(ownNumber) else { return false }
        let response = await DailyAPI.userClient().registerUser(request: .with {
            $0.deviceID = DeviceUser.id
            $0.phoneHash = phoneHash
        })
        return response.error == nil
    }

    /// A contact as read from the address book: a display name, its numbers, and
    /// an optional thumbnail photo.
    public struct Contact: Sendable {
        public let name: String
        public let numbers: [String]
        public let avatar: Data?
        public init(name: String, numbers: [String], avatar: Data? = nil) {
            self.name = name
            self.numbers = numbers
            self.avatar = avatar
        }
    }

    /// Hash every contact number, ask the backend which are registered, and
    /// persist the resulting `Friend` list (joined back to local names) to the
    /// App Group. Returns the friends found.
    @discardableResult
    public static func syncFriends(from contacts: [Contact]) async -> [Friend] {
        // Map each hash back to the contact (name + photo) that produced it.
        var infoByHash: [Data: (name: String, avatar: Data?)] = [:]
        for contact in contacts {
            for number in contact.numbers {
                guard let hash = PhoneHash.hash(number) else { continue }
                // First contact wins a given number; avoids overwriting on dupes.
                if infoByHash[hash] == nil {
                    infoByHash[hash] = (contact.name, contact.avatar)
                }
            }
        }
        guard !infoByHash.isEmpty else {
            FriendStore.save([])
            return []
        }

        let response = await DailyAPI.userClient().matchContacts(request: .with {
            $0.phoneHashes = Array(infoByHash.keys)
        })
        guard let matches = response.message?.matches else { return FriendStore.load() }

        let ownID = DeviceUser.id
        var friends: [Friend] = []
        for match in matches where match.deviceID != ownID {
            let info = infoByHash[match.phoneHash]
            friends.append(Friend(
                deviceID: match.deviceID,
                name: info?.name ?? "Friend",
                avatar: info?.avatar
            ))
        }
        FriendStore.save(friends)
        return friends
    }
}
