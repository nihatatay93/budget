import Testing
@testable import Budget

/// A session token is a bearer credential, so plaintext HTTP is only acceptable where the
/// network path is short enough to be worth it.
@Suite("Server address policy")
struct ServerAddressPolicyTests {
    @Test("Loopback is always plaintext-safe", arguments: ["localhost", "127.0.0.1", "::1", "LOCALHOST"])
    func loopbackAllowed(host: String) {
        #expect(ServerAddressPolicy.allowsPlaintextHTTP(host: host))
    }

    @Test("Private network addresses are recognised", arguments: [
        "192.168.1.230", "10.0.0.5", "172.16.0.1", "172.31.255.254",
        "169.254.1.1", "nihats-mac.local",
    ])
    func privateRangesRecognised(host: String) {
        #expect(ServerAddressPolicy.isPrivateNetwork(host))
    }

    /// "localhost:8080" parses with "localhost" as its scheme and no host, so without a supplied
    /// scheme the app refuses the most natural thing anyone types at it.
    @Test("A missing scheme is supplied", arguments: [
        ("localhost:8080", "http://localhost:8080"),
        ("127.0.0.1:8080", "http://127.0.0.1:8080"),
        ("192.168.1.10:8080", "http://192.168.1.10:8080"),
        ("nihats-mac.local:8080", "http://nihats-mac.local:8080"),
        ("  localhost:8080  ", "http://localhost:8080"),
    ])
    func missingSchemeSupplied(input: String, expected: String) {
        #expect(ServerAddressPolicy.addressWithInferredScheme(input) == expected)
    }

    /// A public host takes https, because supplying http there would quietly put a bearer token
    /// on the wire in the clear — the opposite of what this policy exists to prevent.
    @Test("A public host is never downgraded to plaintext", arguments: [
        ("budget.example.com", "https://budget.example.com"),
        ("budget.example.com:8443", "https://budget.example.com:8443"),
        ("8.8.8.8", "https://8.8.8.8"),
    ])
    func publicHostTakesHTTPS(input: String, expected: String) {
        #expect(ServerAddressPolicy.addressWithInferredScheme(input) == expected)
    }

    @Test("An address that already states its scheme is left alone", arguments: [
        "http://localhost:8080", "https://budget.example.com", "HTTPS://budget.example.com", "",
    ])
    func statedSchemeUntouched(address: String) {
        #expect(ServerAddressPolicy.addressWithInferredScheme(address) == address)
    }

    /// 172.32 is outside RFC 1918, and a public address must never be mistaken for a private
    /// one or a Debug build would send the token across the internet in the clear.
    @Test("Public addresses are not private", arguments: [
        "172.32.0.1", "172.15.0.1", "8.8.8.8", "203.0.113.9",
        "example.com", "192.168.1", "192.168.1.256", "",
    ])
    func publicAddressesRejected(host: String) {
        #expect(ServerAddressPolicy.isPrivateNetwork(host) == false)
    }

    /// The shipped build must not accept plaintext beyond loopback, whatever the network.
    @Test("Release builds refuse plaintext outside loopback")
    func releaseRefusesPrivateNetwork() {
        #if DEBUG
        #expect(ServerAddressPolicy.allowsPlaintextHTTP(host: "192.168.1.230"))
        #else
        #expect(ServerAddressPolicy.allowsPlaintextHTTP(host: "192.168.1.230") == false)
        #endif
        // Public hosts are refused in every build.
        #expect(ServerAddressPolicy.allowsPlaintextHTTP(host: "budget.example.com") == false)
    }
}
