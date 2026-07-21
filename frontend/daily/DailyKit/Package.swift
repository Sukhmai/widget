// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "DailyKit",
    platforms: [
        .iOS(.v17),
    ],
    products: [
        .library(name: "DailyKit", targets: ["DailyKit"]),
    ],
    dependencies: [
        .package(url: "https://github.com/connectrpc/connect-swift", from: "1.2.3"),
        // E.164 phone-number normalization so both parties hash identically.
        .package(url: "https://github.com/marmelroy/PhoneNumberKit", .upToNextMajor(from: "3.7.0")),
    ],
    targets: [
        .target(
            name: "DailyKit",
            dependencies: [
                .product(name: "Connect", package: "connect-swift"),
                .product(name: "PhoneNumberKit", package: "PhoneNumberKit"),
            ],
            resources: [
                .process("Media.xcassets"),
            ]
        ),
    ]
)
