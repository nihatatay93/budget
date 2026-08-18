// Package mail builds and delivers outbound email over SMTP.
//
// Delivery is optional: the application works without it, and a failure to send never fails
// the action that triggered the message.
package mail

import (
	"errors"
	"fmt"
	"mime"
	"net/mail"
	"strings"
	"time"
)

// ErrInvalidMessage reports a message that cannot be built safely.
var ErrInvalidMessage = errors.New("invalid email message")

// Message is a single plain-text email.
type Message struct {
	FromAddress string
	FromName    string
	To          string
	Subject     string
	Body        string
}

// Build renders the message as RFC 5322 bytes.
//
// Every header value is validated rather than escaped. A newline in a display name or
// subject would let the caller append headers of its own -- a Bcc, or a second body -- so an
// address or subject carrying one is rejected outright instead of being repaired into
// something that looks valid.
func (m Message) Build(sentAt time.Time) ([]byte, error) {
	from, err := mail.ParseAddress(m.FromAddress)
	if err != nil {
		return nil, fmt.Errorf("%w: from address", ErrInvalidMessage)
	}
	to, err := mail.ParseAddress(m.To)
	if err != nil {
		return nil, fmt.Errorf("%w: recipient address", ErrInvalidMessage)
	}
	if containsHeaderBreak(m.FromName) || containsHeaderBreak(m.Subject) {
		return nil, fmt.Errorf("%w: header value contains a line break", ErrInvalidMessage)
	}
	if strings.TrimSpace(m.Subject) == "" {
		return nil, fmt.Errorf("%w: subject is empty", ErrInvalidMessage)
	}

	sender := (&mail.Address{Name: m.FromName, Address: from.Address}).String()
	recipient := (&mail.Address{Address: to.Address}).String()

	var builder strings.Builder
	writeHeader(&builder, "From", sender)
	writeHeader(&builder, "To", recipient)
	// Encoded-word keeps a non-ASCII subject readable without emitting raw bytes in a header.
	writeHeader(&builder, "Subject", mime.QEncoding.Encode("utf-8", m.Subject))
	writeHeader(&builder, "Date", sentAt.Format(time.RFC1123Z))
	writeHeader(&builder, "MIME-Version", "1.0")
	writeHeader(&builder, "Content-Type", `text/plain; charset="utf-8"`)
	writeHeader(&builder, "Content-Transfer-Encoding", "8bit")
	// An invitation is a one-off notification, not a subscription, so it asks not to be
	// auto-replied to and not to be indexed by bulk-mail heuristics as a campaign.
	writeHeader(&builder, "Auto-Submitted", "auto-generated")
	builder.WriteString("\r\n")
	builder.WriteString(normalizeBody(m.Body))
	return []byte(builder.String()), nil
}

func writeHeader(builder *strings.Builder, name, value string) {
	builder.WriteString(name)
	builder.WriteString(": ")
	builder.WriteString(value)
	builder.WriteString("\r\n")
}

func containsHeaderBreak(value string) bool {
	return strings.ContainsAny(value, "\r\n")
}

// normalizeBody converts to the CRLF line endings SMTP expects and escapes a line that would
// otherwise be read as the end-of-data marker.
func normalizeBody(body string) string {
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	for index, line := range lines {
		if line == "." {
			lines[index] = ".."
		}
	}
	return strings.Join(lines, "\r\n")
}

// parseAddress returns the bare address for use in an SMTP envelope.
func parseAddress(value string) (string, error) {
	parsed, err := mail.ParseAddress(value)
	if err != nil {
		return "", fmt.Errorf("%w: %q", ErrInvalidMessage, value)
	}
	return parsed.Address, nil
}
