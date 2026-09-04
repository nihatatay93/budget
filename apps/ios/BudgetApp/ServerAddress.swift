import Foundation

/// Where a plaintext HTTP server address is acceptable.
///
/// A session token travels as a bearer credential, so HTTP exposes it to anyone on the path.
/// Loopback has no path to expose. A private network has a short one, which is worth it while
/// developing against a Mac on the same Wi-Fi and not worth it in a shipped build, so those
/// hosts are accepted only in Debug.
enum ServerAddressPolicy {
    static func allowsPlaintextHTTP(host: String) -> Bool {
        if isLoopback(host) { return true }
        #if DEBUG
        return isPrivateNetwork(host)
        #else
        return false
        #endif
    }

    static func isLoopback(_ host: String) -> Bool {
        ["localhost", "127.0.0.1", "::1"].contains(host.lowercased())
    }

    /// RFC 1918 ranges, link-local, and mDNS names, which together cover a machine reachable
    /// only from the same network.
    static func isPrivateNetwork(_ host: String) -> Bool {
        let name = host.lowercased()
        if name.hasSuffix(".local") { return true }

        let octets = name.split(separator: ".", omittingEmptySubsequences: false)
        guard octets.count == 4 else { return false }
        let numbers = octets.compactMap { UInt8($0) }
        guard numbers.count == 4 else { return false }

        switch (numbers[0], numbers[1]) {
        case (10, _): return true
        case (192, 168): return true
        case (172, 16...31): return true
        case (169, 254): return true
        default: return false
        }
    }
}

extension ServerAddressPolicy {
    /// Supplies a missing scheme so a typed address reads the way a person means it.
    ///
    /// "localhost:8080" is the natural thing to type and the worst thing to parse: `URLComponents`
    /// reads "localhost" as the scheme and finds no host at all, so the address is refused before
    /// any connection is attempted and the app looks unreachable rather than misaddressed.
    ///
    /// The scheme chosen is not a coin flip. A loopback or private host takes http, which is
    /// exactly where this policy already permits plaintext; everything else takes https, so a
    /// public host is never silently downgraded to carrying a bearer token in the clear.
    static func addressWithInferredScheme(_ address: String) -> String {
        let trimmed = address.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return trimmed }
        if let scheme = URLComponents(string: trimmed)?.scheme?.lowercased(),
           scheme == "http" || scheme == "https" {
            return trimmed
        }
        // Parsed against a placeholder scheme purely to find the host the person typed.
        guard let host = URLComponents(string: "http://\(trimmed)")?.host, !host.isEmpty else {
            return trimmed
        }
        return (allowsPlaintextHTTP(host: host) ? "http://" : "https://") + trimmed
    }

    /// The message shown when an address is refused. It names what is actually accepted in
    /// this build, so a Debug user is not told to use HTTPS when a LAN address would work.
    static var addressGuidance: String {
        #if DEBUG
        L10n.text("Use an HTTPS URL, or HTTP for localhost or a private network address such as http://192.168.1.10:8080")
        #else
        L10n.text("Use an HTTPS server URL, or HTTP for localhost development.")
        #endif
    }
}
