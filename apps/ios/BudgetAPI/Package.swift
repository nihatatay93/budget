// swift-tools-version: 6.1

import PackageDescription

let package = Package(
    name: "BudgetAPI",
    platforms: [.macOS(.v13), .iOS(.v18)],
    products: [
        .library(name: "BudgetAPI", targets: ["BudgetAPI"]),
    ],
    dependencies: [
        .package(url: "https://github.com/apple/swift-openapi-generator", from: "1.13.0"),
        .package(url: "https://github.com/apple/swift-openapi-runtime", from: "1.11.0"),
        .package(url: "https://github.com/apple/swift-openapi-urlsession", from: "1.3.0"),
        .package(url: "https://github.com/apple/swift-http-types", from: "1.4.0"),
    ],
    targets: [
        .target(
            name: "BudgetAPI",
            dependencies: [
                .product(name: "OpenAPIRuntime", package: "swift-openapi-runtime"),
                .product(name: "OpenAPIURLSession", package: "swift-openapi-urlsession"),
                .product(name: "HTTPTypes", package: "swift-http-types"),
            ],
            plugins: [
                .plugin(name: "OpenAPIGenerator", package: "swift-openapi-generator"),
            ]
        ),
        .testTarget(
            name: "BudgetAPITests",
            dependencies: ["BudgetAPI"]
        ),
    ]
)
