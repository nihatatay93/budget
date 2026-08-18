//go:build integration

package mail

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// relay speaks just enough SMTP to accept one message, so the sender is exercised against a
// real protocol conversation rather than a mock shaped to match it.
type relay struct {
	listener  net.Listener
	tlsConfig *tls.Config
	roots     *x509.CertPool
	offerTLS  bool

	mu       sync.Mutex
	received string
	authLine string
	usedTLS  bool
}

func newRelay(t *testing.T, offerTLS bool) *relay {
	t.Helper()
	certificate, roots := selfSignedCertificate(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	instance := &relay{
		listener:  listener,
		tlsConfig: &tls.Config{Certificates: []tls.Certificate{certificate}},
		roots:     roots,
		offerTLS:  offerTLS,
	}
	t.Cleanup(func() { _ = listener.Close() })
	go instance.serve()
	return instance
}

func (r *relay) port() int { return r.listener.Addr().(*net.TCPAddr).Port }

func (r *relay) serve() {
	connection, err := r.listener.Accept()
	if err != nil {
		return
	}
	defer func() { _ = connection.Close() }()
	_ = connection.SetDeadline(time.Now().Add(10 * time.Second))

	write := func(line string) { _, _ = connection.Write([]byte(line + "\r\n")) }
	write("220 relay ESMTP")

	buffer := make([]byte, 8192)
	inData := false
	var body strings.Builder
	for {
		count, err := connection.Read(buffer)
		if err != nil {
			return
		}
		chunk := strings.TrimSuffix(string(buffer[:count]), "\r\n")
		for _, line := range strings.Split(chunk, "\r\n") {
			if inData {
				if line == "." {
					inData = false
					r.mu.Lock()
					r.received = body.String()
					r.mu.Unlock()
					write("250 accepted")
					continue
				}
				body.WriteString(line + "\n")
				continue
			}
			switch {
			case strings.HasPrefix(line, "EHLO"):
				if r.offerTLS {
					write("250-relay")
					write("250-STARTTLS")
					write("250 AUTH PLAIN")
				} else {
					write("250 relay")
				}
			case strings.HasPrefix(line, "STARTTLS"):
				write("220 ready")
				secured := tls.Server(connection, r.tlsConfig)
				if err := secured.Handshake(); err != nil {
					return
				}
				r.mu.Lock()
				r.usedTLS = true
				r.mu.Unlock()
				connection = secured
				_ = connection.SetDeadline(time.Now().Add(10 * time.Second))
				write = func(line string) { _, _ = connection.Write([]byte(line + "\r\n")) }
			case strings.HasPrefix(line, "AUTH"):
				r.mu.Lock()
				r.authLine = line
				r.mu.Unlock()
				write("235 authenticated")
			case strings.HasPrefix(line, "MAIL FROM"), strings.HasPrefix(line, "RCPT TO"):
				write("250 ok")
			case strings.HasPrefix(line, "DATA"):
				inData = true
				write("354 send it")
			case strings.HasPrefix(line, "QUIT"):
				write("221 bye")
				return
			default:
				write("250 ok")
			}
		}
	}
}

func selfSignedCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(
		&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)},
	)
	certificate, err := tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil {
		t.Fatalf("build key pair: %v", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certificatePEM) {
		t.Fatal("add relay certificate to the trust pool")
	}
	return certificate, roots
}

// The whole path: STARTTLS is negotiated, credentials follow only after that, and the message
// arrives with its headers and the invitation code intact.
func TestSenderDeliversOverSTARTTLS(t *testing.T) {
	instance := newRelay(t, true)
	sender := NewSender(Options{
		Host: "127.0.0.1", Port: instance.port(),
		Username: "budget@example.com", Password: "app-password",
		Timeout: 10 * time.Second,
		// The relay signs its own certificate, which is the case a self-hoster with an
		// internal authority is in; verification still happens against this pool.
		RootCAs: instance.roots,
		Now:     func() time.Time { return time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC) },
	})

	if err := sender.Send(Message{
		FromAddress: "budget@example.com",
		FromName:    "Budget",
		To:          "invited@example.com",
		Subject:     "Owner invited you to Atay Family",
		Body:        "Paste this code:\n\n    single-use-token\n",
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	instance.mu.Lock()
	defer instance.mu.Unlock()
	if !instance.usedTLS {
		t.Fatal("the message was sent without negotiating TLS")
	}
	// net/smtp refuses PLAIN on an unencrypted connection, so this arriving at all confirms
	// the credential followed the upgrade rather than preceding it.
	if !strings.HasPrefix(instance.authLine, "AUTH PLAIN") {
		t.Fatalf("auth line = %q, want AUTH PLAIN after TLS", instance.authLine)
	}
	for _, expected := range []string{
		`From: "Budget" <budget@example.com>`,
		"To: <invited@example.com>",
		"single-use-token",
	} {
		if !strings.Contains(instance.received, expected) {
			t.Fatalf("delivered message missing %q:\n%s", expected, instance.received)
		}
	}
}

// A relay that does not offer STARTTLS is refused: the connection would otherwise carry both
// the account credential and the invitation token in the clear.
func TestSenderRefusesAPlaintextRelay(t *testing.T) {
	instance := newRelay(t, false)
	sender := NewSender(Options{
		Host: "127.0.0.1", Port: instance.port(),
		Username: "budget@example.com", Password: "app-password",
		Timeout: 5 * time.Second,
	})

	err := sender.Send(Message{
		FromAddress: "budget@example.com", FromName: "Budget",
		To: "invited@example.com", Subject: "Invitation", Body: "code\n",
	})
	if err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("Send() error = %v, want a STARTTLS refusal", err)
	}
}
