import Testing
import Foundation
@testable import DailyKit

/// Each test gets its own throwaway `UserDefaults` suite (via the `defaults:`
/// parameter `StoryProgress` accepts) so tests never touch the real App Group
/// storage and can't interfere with each other.
private func makeDefaults() -> UserDefaults {
    let suiteName = "DailyKitTests.\(UUID().uuidString)"
    let defaults = UserDefaults(suiteName: suiteName)!
    defaults.removePersistentDomain(forName: suiteName)
    return defaults
}

@Suite("StoryProgress")
struct StoryProgressTests {
    let scope = StoryScope.friend("friend-a")
    let otherScope = StoryScope.friend("friend-b")

    @Test("first resolve bootstraps the stack, top is the newest atom")
    func bootstrap() {
        let defaults = makeDefaults()
        let top = StoryProgress.resolveTop(freshAtomIDs: ["c", "b", "a"], scope: scope, defaults: defaults)
        #expect(top == "c")
    }

    @Test("immediately re-resolving without a pop request or elapsed time doesn't move the top")
    func noPopWithoutGating() {
        let defaults = makeDefaults()
        StoryProgress.resolveTop(freshAtomIDs: ["c", "b", "a"], scope: scope, defaults: defaults)
        let top = StoryProgress.resolveTop(freshAtomIDs: ["c", "b", "a"], scope: scope, defaults: defaults)
        #expect(top == "c")
    }

    @Test("requestPop pops exactly the requested scope")
    func activePopIsScoped() {
        let defaults = makeDefaults()
        StoryProgress.resolveTop(freshAtomIDs: ["c", "b", "a"], scope: scope, defaults: defaults)
        StoryProgress.resolveTop(freshAtomIDs: ["z", "y"], scope: otherScope, defaults: defaults)

        StoryProgress.requestPop(for: scope, defaults: defaults)

        let poppedTop = StoryProgress.resolveTop(freshAtomIDs: ["c", "b", "a"], scope: scope, defaults: defaults)
        #expect(poppedTop == "b")

        // otherScope never had a pop requested, so a reload of it shouldn't advance.
        let untouchedTop = StoryProgress.resolveTop(freshAtomIDs: ["z", "y"], scope: otherScope, defaults: defaults)
        #expect(untouchedTop == "z")
    }

    @Test("a brand new post jumps straight to the top, mid-lap")
    func newPostJumpsQueue() {
        let defaults = makeDefaults()
        StoryProgress.resolveTop(freshAtomIDs: ["c", "b", "a"], scope: scope, defaults: defaults)
        StoryProgress.requestPop(for: scope, defaults: defaults)
        let afterPop = StoryProgress.resolveTop(freshAtomIDs: ["c", "b", "a"], scope: scope, defaults: defaults)
        #expect(afterPop == "b")

        // "d" was posted after the last check — it should jump ahead of "b",
        // not wait for the lap to wrap back around to it.
        let afterNewPost = StoryProgress.resolveTop(freshAtomIDs: ["d", "c", "b", "a"], scope: scope, defaults: defaults)
        #expect(afterNewPost == "d")
    }

    @Test("popping through the whole lap refills and loops back to the newest")
    func emptyStackRefills() {
        let defaults = makeDefaults()
        let atoms = ["c", "b", "a"]
        StoryProgress.resolveTop(freshAtomIDs: atoms, scope: scope, defaults: defaults)

        for _ in 0..<atoms.count {
            StoryProgress.requestPop(for: scope, defaults: defaults)
            StoryProgress.resolveTop(freshAtomIDs: atoms, scope: scope, defaults: defaults)
        }

        // Popped past "a" (the last entry) — should already be back on "c".
        #expect(StoryProgress.top(for: scope, defaults: defaults) == "c")
    }

    @Test("an atom that ages out of the active window is dropped from the stack")
    func expiredAtomIsPruned() {
        let defaults = makeDefaults()
        StoryProgress.resolveTop(freshAtomIDs: ["c", "b", "a"], scope: scope, defaults: defaults)
        StoryProgress.requestPop(for: scope, defaults: defaults)
        StoryProgress.resolveTop(freshAtomIDs: ["c", "b", "a"], scope: scope, defaults: defaults)
        // top is now "b"; "a" expired out of the 24h window entirely.
        let top = StoryProgress.resolveTop(freshAtomIDs: ["c", "b"], scope: scope, defaults: defaults)
        #expect(top == "b")
        StoryProgress.requestPop(for: scope, defaults: defaults)
        let next = StoryProgress.resolveTop(freshAtomIDs: ["c", "b"], scope: scope, defaults: defaults)
        // "a" should never resurface since it's gone from the fresh fetch.
        #expect(next != "a")
    }

    @Test("an atom already tapped-viewed via another scope isn't pushed as new")
    func viewedAtomIsNotDuplicated() {
        let defaults = makeDefaults()
        StoryProgress.markViewed("shared-atom", defaults: defaults)

        // Even on a brand new scope's very first bootstrap, an already
        // tapped-viewed atom is excluded — "older" becomes top instead.
        let top = StoryProgress.resolveTop(freshAtomIDs: ["shared-atom", "older"], scope: scope, defaults: defaults)
        #expect(top == "older")

        // Same when it shows up later, mid-lap, on a different scope.
        let freshStack = StoryProgress.resolveTop(freshAtomIDs: ["older"], scope: otherScope, defaults: defaults)
        #expect(freshStack == "older")
        let afterViewedAppears = StoryProgress.resolveTop(
            freshAtomIDs: ["shared-atom", "older"], scope: otherScope, defaults: defaults
        )
        #expect(afterViewedAppears == "older")
    }
}
