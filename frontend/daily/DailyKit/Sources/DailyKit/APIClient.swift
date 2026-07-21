import Connect
import Foundation

/// Builds connect-rpc clients pointed at the configured API base URL.
///
/// The base URL comes from the `APIBaseURL` Info.plist key, which is populated
/// from the `API_BASE_URL` build setting (Debug = localhost, Release = prod).
public enum DailyAPI {
    /// The API host, e.g. `http://localhost:8080`. Falls back to localhost when
    /// the Info.plist key is missing (e.g. SwiftUI previews).
    public static var baseURL: String {
        let value = Bundle.main.object(forInfoDictionaryKey: "APIBaseURL") as? String
        if let value, !value.isEmpty, value != "$(API_BASE_URL)" {
            return value
        }
        return "http://localhost:8080"
    }

    public static func protocolClient() -> ProtocolClient {
        ProtocolClient(
            httpClient: URLSessionHTTPClient(),
            config: ProtocolClientConfig(
                host: baseURL,
                networkProtocol: .connect,
                codec: ProtoCodec()
            )
        )
    }

    public static func atomClient() -> Atom_V1_AtomServiceClient {
        Atom_V1_AtomServiceClient(client: protocolClient())
    }
}
