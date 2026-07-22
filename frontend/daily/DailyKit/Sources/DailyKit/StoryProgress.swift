import Foundation

/// Which set of atoms a widget instance is cycling through: one pinned
/// friend's, or the merged feed across every subscribed friend.
public enum StoryScope: Hashable, Codable, Sendable {
    case friend(String)
    case merged

    /// Stable string key for UserDefaults storage.
    var key: String {
        switch self {
        case .friend(let id): return "friend:\(id)"
        case .merged: return "merged"
        }
    }

    /// Inverse of `key`, for reconstructing a scope from stored state.
    static func from(key: String) -> StoryScope? {
        if key == "merged" { return .merged }
        if key.hasPrefix("friend:") { return .friend(String(key.dropFirst("friend:".count))) }
        return nil
    }
}

/// Cross-process cycling state for the story widget, persisted in the App
/// Group so both the widget extension (which pushes/pops on refresh) and the
/// app (which requests a pop from `onOpenURL`) see the same state.
///
/// Each scope owns a stack of not-yet-popped atom ids, newest on top. The
/// widget always shows the top. Popping (a matching tap, or `passiveInterval`
/// having elapsed) removes the top, revealing the next-older atom. A brand
/// new post is pushed onto the top as soon as it's discovered, so it's shown
/// next regardless of how far into the current lap the stack already is.
/// When the stack empties, it's refilled from the full active (last-24h)
/// atom list, newest on top — this is what makes finishing a lap loop back
/// to the newest atom instead of going empty.
///
/// Every function takes a `defaults` parameter (default `AppGroup.defaults`)
/// purely so tests can inject an isolated `UserDefaults` suite instead of
/// touching the real App Group storage.
public enum StoryProgress {
    /// How often a scope may pop on its own (no matching tap).
    /// Matches the widget's own timeline refresh cadence in story.swift.
    public static let passiveInterval: TimeInterval = 15 * 60

    private static let stackKey = "DailyKit.storyStacks"
    private static let knownKey = "DailyKit.storyKnownAtomIDs"
    private static let topURLKey = "DailyKit.storyTopURLs"
    private static let lastPoppedKey = "DailyKit.storyLastPopped"
    private static let pendingPopKey = "DailyKit.storyPendingPop"
    private static let viewedKey = "DailyKit.viewedAtomIDs"

    // MARK: Stack storage (top = index 0)

    private static func stacks(_ defaults: UserDefaults) -> [String: [String]] {
        guard let data = defaults.data(forKey: stackKey),
              let decoded = try? JSONDecoder().decode([String: [String]].self, from: data) else {
            return [:]
        }
        return decoded
    }

    /// The atom id currently on top of a scope's stack, if any — i.e. what's
    /// currently displayed for that scope, without needing a fresh fetch.
    public static func top(for scope: StoryScope, defaults: UserDefaults = AppGroup.defaults) -> String? {
        stacks(defaults)[scope.key]?.first
    }

    private static func setStack(_ stack: [String], for scope: StoryScope, _ defaults: UserDefaults) {
        var all = stacks(defaults)
        all[scope.key] = stack
        guard let data = try? JSONEncoder().encode(all) else { return }
        defaults.set(data, forKey: stackKey)
    }

    /// Every atom id ever pushed onto a scope's stack, whether still resident
    /// or already popped — lets us tell "genuinely new post" apart from "an
    /// atom we already popped past, still present in a fresh fetch."
    private static func known(_ defaults: UserDefaults) -> [String: Set<String>] {
        guard let data = defaults.data(forKey: knownKey),
              let decoded = try? JSONDecoder().decode([String: Set<String>].self, from: data) else {
            return [:]
        }
        return decoded
    }

    private static func setKnown(_ ids: Set<String>, for scope: StoryScope, _ defaults: UserDefaults) {
        var all = known(defaults)
        all[scope.key] = ids
        guard let data = try? JSONEncoder().encode(all) else { return }
        defaults.set(data, forKey: knownKey)
    }

    // MARK: Top URL (for matching a tapped URL back to its scope)

    /// The resource URL currently on top for a given scope, if any. Recorded
    /// separately from the atom id (set by `story.swift` once it resolves the
    /// displayed atom's link) so a widget tap can be matched back to the
    /// scope(s) it came from — see `scopesShowing(url:)`.
    public static func setTopURL(_ url: String, for scope: StoryScope, defaults: UserDefaults = AppGroup.defaults) {
        var all = topURLs(defaults)
        all[scope.key] = url
        guard let data = try? JSONEncoder().encode(all) else { return }
        defaults.set(data, forKey: topURLKey)
    }

    private static func topURLs(_ defaults: UserDefaults) -> [String: String] {
        guard let data = defaults.data(forKey: topURLKey),
              let decoded = try? JSONDecoder().decode([String: String].self, from: data) else {
            return [:]
        }
        return decoded
    }

    /// Every scope currently known to be showing the given URL (usually one,
    /// but resources are deduplicated by URL server-side, so two friends who
    /// shared the same link can collide here — both are returned).
    public static func scopesShowing(url: String, defaults: UserDefaults = AppGroup.defaults) -> [StoryScope] {
        topURLs(defaults).compactMap { key, storedURL in
            guard storedURL == url else { return nil }
            return StoryScope.from(key: key)
        }
    }

    // MARK: Pop gating (active tap request vs. passive interval)

    /// Request that a scope pop the next time its Provider renders,
    /// regardless of `passiveInterval`. Set only for the scope(s) a tap's URL
    /// actually matched (see `scopesShowing(url:)`) — this is what lets a tap
    /// on one widget instance pop just that instance's scope, even though
    /// `WidgetCenter.reloadTimelines(ofKind:)` re-renders every instance of
    /// the "story" kind, including unrelated scopes.
    public static func requestPop(for scope: StoryScope, defaults: UserDefaults = AppGroup.defaults) {
        var pending = pendingPopScopeKeys(defaults)
        pending.insert(scope.key)
        guard let data = try? JSONEncoder().encode(pending) else { return }
        defaults.set(data, forKey: pendingPopKey)
    }

    private static func consumePendingPop(for scope: StoryScope, _ defaults: UserDefaults) -> Bool {
        var pending = pendingPopScopeKeys(defaults)
        guard pending.remove(scope.key) != nil else { return false }
        guard let data = try? JSONEncoder().encode(pending) else { return true }
        defaults.set(data, forKey: pendingPopKey)
        return true
    }

    private static func pendingPopScopeKeys(_ defaults: UserDefaults) -> Set<String> {
        guard let data = defaults.data(forKey: pendingPopKey),
              let decoded = try? JSONDecoder().decode(Set<String>.self, from: data) else {
            return []
        }
        return decoded
    }

    private static func lastPoppedAt(for scope: StoryScope, _ defaults: UserDefaults) -> Date? {
        guard let data = defaults.data(forKey: lastPoppedKey),
              let decoded = try? JSONDecoder().decode([String: TimeInterval].self, from: data),
              let timestamp = decoded[scope.key] else {
            return nil
        }
        return Date(timeIntervalSince1970: timestamp)
    }

    private static func markPoppedNow(for scope: StoryScope, _ defaults: UserDefaults) {
        var all: [String: TimeInterval] = [:]
        if let data = defaults.data(forKey: lastPoppedKey) {
            all = (try? JSONDecoder().decode([String: TimeInterval].self, from: data)) ?? [:]
        }
        all[scope.key] = Date().timeIntervalSince1970
        guard let data = try? JSONEncoder().encode(all) else { return }
        defaults.set(data, forKey: lastPoppedKey)
    }

    /// `false` when this scope has never popped — "no history" means "just
    /// started," not "overdue." `resolveTop` starts the clock on first
    /// observation instead, so this only ever sees a real elapsed duration.
    private static func isPassiveDue(_ scope: StoryScope, _ defaults: UserDefaults) -> Bool {
        guard let last = lastPoppedAt(for: scope, defaults) else { return false }
        return Date().timeIntervalSince(last) >= passiveInterval
    }

    // MARK: Resolving what to show

    /// Reconciles a scope's stack against a freshly-fetched, newest-first
    /// list of active (last-24h) atom ids, then returns the atom id that
    /// should be displayed:
    ///   1. Any atom id in `freshAtomIDs` that hasn't been seen before (a
    ///      genuinely new post) *and* hasn't already been tapped-viewed via a
    ///      different scope (see `markViewed`) is pushed onto the top of the
    ///      stack — newest arrival ends up on top — so a new post is shown
    ///      next regardless of how deep into the current lap the stack
    ///      already is, and something you already tapped through in one
    ///      scope (e.g. a pinned friend) doesn't duplicate into another
    ///      (e.g. the merged "All" feed).
    ///   2. Any stack entry no longer present in `freshAtomIDs` (deleted, or
    ///      aged out of the 24h window) is dropped.
    ///   3. If the stack is empty (to start, or every entry just got
    ///      pruned), it's refilled from the full fresh list, unfiltered —
    ///      starting a new lap from the newest atom. This is deliberately
    ///      not viewed-filtered: it's both the "loop back to newest" refill
    ///      and the fallback that guarantees the widget always has something
    ///      to show even if every active atom happens to already be viewed.
    ///   4. The top is popped (revealing the next-older atom) only if a tap
    ///      requested it for this scope, or `passiveInterval` has elapsed
    ///      since this scope last popped — otherwise the current top is left
    ///      as-is, so a reload triggered by a *different* scope's tap
    ///      doesn't nudge this one forward early. Popping the very last
    ///      entry immediately starts the next lap rather than showing
    ///      nothing.
    @discardableResult
    public static func resolveTop(
        freshAtomIDs: [String],
        scope: StoryScope,
        defaults: UserDefaults = AppGroup.defaults
    ) -> String? {
        guard !freshAtomIDs.isEmpty else { return nil }
        let freshSet = Set(freshAtomIDs)

        // Start this scope's passive-pop clock the first time it's ever
        // observed. Without this, "never popped" would read as "overdue"
        // (see isPassiveDue) and the very first render would immediately pop
        // past the true newest atom before anyone saw it.
        if lastPoppedAt(for: scope, defaults) == nil {
            markPoppedNow(for: scope, defaults)
        }

        var stack = stacks(defaults)[scope.key] ?? []
        var seen = known(defaults)[scope.key] ?? []

        // Push genuinely new arrivals onto the top, skipping anything already
        // tapped-viewed elsewhere. freshAtomIDs is already newest-first, so
        // filtering preserves that order within the batch — prepending it as
        // -is puts the very newest arrival on top.
        let viewed = viewedAtomIDs(defaults: defaults)
        let newIDs = freshAtomIDs.filter { !seen.contains($0) && !viewed.contains($0) }
        if !newIDs.isEmpty {
            stack = newIDs + stack
            seen.formUnion(newIDs)
        }

        // Drop anything that's fallen out of the active window.
        stack.removeAll { !freshSet.contains($0) }

        if stack.isEmpty {
            stack = freshAtomIDs
            seen = freshSet
        } else {
            let requestedPop = consumePendingPop(for: scope, defaults)
            if requestedPop || isPassiveDue(scope, defaults) {
                stack.removeFirst()
                markPoppedNow(for: scope, defaults)
                if stack.isEmpty {
                    stack = freshAtomIDs
                }
            }
        }

        setStack(stack, for: scope, defaults)
        setKnown(seen, for: scope, defaults)
        return stack.first
    }

    // MARK: Viewed tracking (tap-only; also consulted by resolveTop to avoid
    // duplicating tapped content across scopes)

    public static func markViewed(_ atomID: String, defaults: UserDefaults = AppGroup.defaults) {
        var ids = viewedAtomIDs(defaults: defaults)
        ids.insert(atomID)
        guard let data = try? JSONEncoder().encode(ids) else { return }
        defaults.set(data, forKey: viewedKey)
    }

    public static func viewedAtomIDs(defaults: UserDefaults = AppGroup.defaults) -> Set<String> {
        guard let data = defaults.data(forKey: viewedKey),
              let decoded = try? JSONDecoder().decode(Set<String>.self, from: data) else {
            return []
        }
        return decoded
    }
}
