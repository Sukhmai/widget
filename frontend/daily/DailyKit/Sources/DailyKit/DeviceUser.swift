import Foundation
#if canImport(UIKit)
import UIKit
#endif

/// The device's stable user identifier. `identifierForVendor` returns the same
/// UUID for an app and its extensions (same vendor), so the `daily` app and the
/// `story` widget share one id with no App Group. A persisted UserDefaults UUID
/// is used only in the rare case IDFV is nil.
public enum DeviceUser {
    private static let fallbackKey = "DailyKit.deviceUserID"

    public static var id: String {
        #if canImport(UIKit)
        if let idfv = UIDevice.current.identifierForVendor?.uuidString {
            return idfv
        }
        #endif
        let defaults = UserDefaults.standard
        if let existing = defaults.string(forKey: fallbackKey) {
            return existing
        }
        let generated = UUID().uuidString
        defaults.set(generated, forKey: fallbackKey)
        return generated
    }
}
