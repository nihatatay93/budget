package mail

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"time"
)

// Sender delivers messages through an SMTP relay.
//
// The connection is always encrypted: implicit TLS on port 465, STARTTLS otherwise, and a
// relay that offers neither is refused rather than downgraded. Credentials are never sent in
// the clear, which net/smtp also enforces for PLAIN auth.
type Sender struct {
	host     string
	port     int
	username string
	password string
	timeout  time.Duration
	rootCAs  *x509.CertPool
	now      func() time.Time
}

type Options struct {
	Host     string
	Port     int
	Username string
	Password string
	Timeout  time.Duration
	// RootCAs overrides the system trust store, for a relay whose certificate is issued by an
	// internal authority. Nil uses the system roots; verification is never skipped.
	RootCAs *x509.CertPool
	Now     func() time.Time
}

func NewSender(options Options) *Sender {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Sender{
		host:     options.Host,
		port:     options.Port,
		username: options.Username,
		password: options.Password,
		timeout:  options.Timeout,
		rootCAs:  options.RootCAs,
		now:      now,
	}
}

// Send builds and delivers one message.
func (s *Sender) Send(message Message) error {
	payload, err := message.Build(s.now())
	if err != nil {
		return err
	}
	from, to, err := envelope(message)
	if err != nil {
		return err
	}

	client, err := s.connect()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	if s.username != "" {
		auth := smtp.PlainAuth("", s.username, s.password, s.host)
		if err := client.Auth(auth); err != nil {
			// The error can carry the relay's response; the password is never part of it, but
			// the message is still not logged by callers at anything above debug.
			return fmt.Errorf("smtp authenticate: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp sender: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := writer.Write(payload); err != nil {
		_ = writer.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp complete: %w", err)
	}
	return client.Quit()
}

func (s *Sender) connect() (*smtp.Client, error) {
	address := net.JoinHostPort(s.host, strconv.Itoa(s.port))
	dialer := &net.Dialer{Timeout: s.timeout}
	tlsConfig := &tls.Config{
		ServerName: s.host, MinVersion: tls.VersionTLS12, RootCAs: s.rootCAs,
	}

	// Port 465 speaks TLS from the first byte; everything else negotiates with STARTTLS.
	if s.port == 465 {
		connection, err := tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("smtp connect: %w", err)
		}
		client, err := smtp.NewClient(connection, s.host)
		if err != nil {
			_ = connection.Close()
			return nil, fmt.Errorf("smtp handshake: %w", err)
		}
		return client, nil
	}

	connection, err := dialer.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("smtp connect: %w", err)
	}
	if err := connection.SetDeadline(time.Now().Add(s.timeout)); err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("smtp deadline: %w", err)
	}
	client, err := smtp.NewClient(connection, s.host)
	if err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("smtp handshake: %w", err)
	}
	if ok, _ := client.Extension("STARTTLS"); !ok {
		_ = client.Close()
		// Refusing beats sending a credential and an invitation token over plaintext.
		return nil, errors.New("smtp relay does not support STARTTLS")
	}
	if err := client.StartTLS(tlsConfig); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("smtp start TLS: %w", err)
	}
	return client, nil
}

func envelope(message Message) (from, to string, err error) {
	parsedFrom, err := parseAddress(message.FromAddress)
	if err != nil {
		return "", "", err
	}
	parsedTo, err := parseAddress(message.To)
	if err != nil {
		return "", "", err
	}
	return parsedFrom, parsedTo, nil
}
