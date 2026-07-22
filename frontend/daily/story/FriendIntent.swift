//
//  FriendIntent.swift
//  story
//
//  Widget configuration: pick a specific matched friend, or "All" for the
//  merged feed across every matched friend. Options come from the App Group
//  friend list the app populates via contact matching.
//

import AppIntents
import DailyKit

/// Sentinel id for the "All" (merged feed) option — distinct from any real device UUID.
enum FriendSelection {
    static let randomID = "__random__"
}

struct FriendEntity: AppEntity {
    let id: String
    let name: String

    static var typeDisplayRepresentation: TypeDisplayRepresentation { "Friend" }
    static let defaultQuery = FriendQuery()

    var displayRepresentation: DisplayRepresentation { DisplayRepresentation(title: "\(name)") }

    static let random = FriendEntity(id: FriendSelection.randomID, name: "🗞️ All")
}

struct FriendQuery: EntityQuery {
    /// Resolve stored selections back to entities (e.g. a pinned friend).
    func entities(for identifiers: [String]) async throws -> [FriendEntity] {
        identifiers.compactMap { id in
            if id == FriendSelection.randomID { return .random }
            guard let friend = FriendStore.friend(withID: id) else { return nil }
            return FriendEntity(id: friend.deviceID, name: friend.name)
        }
    }

    /// The picker list: All first, then each matched friend.
    func suggestedEntities() async throws -> [FriendEntity] {
        [.random] + FriendStore.load().map { FriendEntity(id: $0.deviceID, name: $0.name) }
    }

    func defaultResult() -> FriendEntity? { .random }
}

struct SelectFriendIntent: WidgetConfigurationIntent {
    static var title: LocalizedStringResource { "Select Friend" }
    static var description: IntentDescription { "Choose whose links to show, or see everyone's." }

    @Parameter(title: "Friend")
    var friend: FriendEntity?
}
