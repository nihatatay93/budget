import Foundation
import HTTPTypes
import OpenAPIRuntime
import OpenAPIURLSession

public enum GeneratedAPI {
    public static func client(serverURL: URL, bearerToken: String? = nil) -> Client {
        let middlewares: [any ClientMiddleware]
        if let bearerToken {
            middlewares = [BearerMiddleware(token: bearerToken)]
        } else {
            middlewares = []
        }
        return Client(
            serverURL: serverURL,
            configuration: .init(dateTranscoder: FlexibleISO8601DateTranscoder()),
            transport: URLSessionTransport(),
            middlewares: middlewares
        )
    }
}

/// PostgreSQL timestamps serialized by Go use RFC 3339 with fractional seconds whenever
/// microseconds are present, but omit the fraction for exact whole seconds. Swift OpenAPI
/// Runtime's two built-in transcoders each accept only one of those valid representations.
final class FlexibleISO8601DateTranscoder: DateTranscoder, @unchecked Sendable {
    private let lock = NSLock()
    private let fractional: ISO8601DateFormatter
    private let wholeSeconds: ISO8601DateFormatter

    init() {
        fractional = ISO8601DateFormatter()
        fractional.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        wholeSeconds = ISO8601DateFormatter()
        wholeSeconds.formatOptions = [.withInternetDateTime]
    }

    func encode(_ date: Date) throws -> String {
        lock.lock()
        defer { lock.unlock() }
        return fractional.string(from: date)
    }

    func decode(_ value: String) throws -> Date {
        lock.lock()
        defer { lock.unlock() }
        if let date = fractional.date(from: value) ?? wholeSeconds.date(from: value) {
            return date
        }
        throw DecodingError.dataCorrupted(.init(
            codingPath: [],
            debugDescription: "Expected an RFC 3339 timestamp with optional fractional seconds."
        ))
    }
}

private struct BearerMiddleware: ClientMiddleware {
    let token: String

    func intercept(
        _ request: HTTPRequest,
        body: HTTPBody?,
        baseURL: URL,
        operationID: String,
        next: (HTTPRequest, HTTPBody?, URL) async throws -> (HTTPResponse, HTTPBody?)
    ) async throws -> (HTTPResponse, HTTPBody?) {
        var authenticatedRequest = request
        authenticatedRequest.headerFields[.authorization] = "Bearer \(token)"
        return try await next(authenticatedRequest, body, baseURL)
    }
}
