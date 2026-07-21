import CryptoKit
import Foundation
import PhoneNumberKit

/// Turns a raw phone number into the opaque discovery key stored in the backend
/// `users.phone_hash` column. Two devices must produce the *same* bytes for the
/// same real number, so normalization (E.164 via PhoneNumberKit) and the HMAC
/// key are fixed and shared by every install.
///
/// The key ships inside the app binary, so these hashes are not secret against a
/// determined attacker who extracts it — acceptable for a friends MVP, and the
/// reason writes stay bound to the device UUID rather than the hash.
public enum PhoneHash {
    /// Shared HMAC key. Rotating this invalidates every existing match, so treat
    /// it as a permanent protocol constant, not a rotating secret.
    private static let hmacKey = SymmetricKey(data: Data("daily.phone-hash.v1".utf8))

    private static let utility = PhoneNumberKit()

    /// Region used when a contact number has no country code (most do not).
    private static var defaultRegion: String {
        Locale.current.region?.identifier ?? "US"
    }

    /// Normalize `raw` to E.164, or return nil if it isn't a valid number.
    public static func normalize(_ raw: String) -> String? {
        guard let parsed = try? utility.parse(raw, withRegion: defaultRegion) else {
            return nil
        }
        return utility.format(parsed, toType: .e164)
    }

    /// HMAC-SHA256 of the E.164 form, or nil for an unparseable number.
    public static func hash(_ raw: String) -> Data? {
        guard let e164 = normalize(raw) else { return nil }
        let mac = HMAC<SHA256>.authenticationCode(for: Data(e164.utf8), using: hmacKey)
        return Data(mac)
    }
}
