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
    ],
    targets: [
        .target(
            name: "DailyKit",
            dependencies: [
                .product(name: "Connect", package: "connect-swift"),
            ],
            resources: [
                .process("Media.xcassets"),
            ]
        ),
    ]
)
