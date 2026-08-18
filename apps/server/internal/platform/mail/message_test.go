package mail

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var sentAt = time.Date(2026, 8, 18, 9, 30, 0, 0, time.UTC)

func validMessage() Message {
	return Message{
		FromAddress: "budget@example.com",
		FromName:    "Budget",
		To:          "invited@example.com",
		Subject:     "Owner invited you to Atay Family",
		Body:        "Paste this code:\n\n    a-token\n",
	}
}

func TestBuildProducesAWellFormedMessage(t *testing.T) {
	built, err := validMessage().Build(sentAt)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	rendered := string(built)

	for _, header := range []string{
		"From: \"Budget\" <budget@example.com>\r\n",
		"To: <invited@example.com>\r\n",
		"MIME-Version: 1.0\r\n",
		"Content-Type: text/plain; charset=\"utf-8\"\r\n",
		"Auto-Submitted: auto-generated\r\n",
	} {
		if !strings.Contains(rendered, header) {
			t.Fatalf("missing header %q in:\n%s", header, rendered)
		}
	}
	// Headers and body are separated by a blank line, and the body survives intact.
	headers, body, found := strings.Cut(rendered, "\r\n\r\n")
	if !found || strings.Contains(headers, "Paste this code") {
		t.Fatalf("headers and body are not separated:\n%s", rendered)
	}
	if !strings.Contains(body, "a-token") {
		t.Fatalf("body lost its content:\n%s", body)
	}
	// Every line ends CRLF, which SMTP requires.
	if strings.Contains(strings.ReplaceAll(rendered, "\r\n", ""), "\n") {
		t.Fatalf("message contains a bare newline:\n%q", rendered)
	}
}

// A newline in a header value would let the caller append headers of its own. A display name
// and subject both come from user-controlled data, so both are refused rather than repaired.
func TestBuildRejectsHeaderInjection(t *testing.T) {
	tests := map[string]func(Message) Message{
		"newline in display name": func(m Message) Message {
			m.FromName = "Budget\r\nBcc: attacker@example.com"
			return m
		},
		"newline in subject": func(m Message) Message {
			m.Subject = "Invitation\r\nBcc: attacker@example.com"
			return m
		},
		"bare linefeed in subject": func(m Message) Message {
			m.Subject = "Invitation\nX-Injected: yes"
			return m
		},
		"newline in recipient": func(m Message) Message {
			m.To = "invited@example.com\r\nBcc: attacker@example.com"
			return m
		},
		"recipient is not an address": func(m Message) Message {
			m.To = "not-an-address"
			return m
		},
		"empty subject": func(m Message) Message {
			m.Subject = "   "
			return m
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := mutate(validMessage()).Build(sentAt); !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("Build() error = %v, want ErrInvalidMessage", err)
			}
		})
	}
}

// A lone dot ends the DATA command, so a body line consisting of one must be escaped or the
// message would be truncated there.
func TestBuildEscapesTheEndOfDataMarker(t *testing.T) {
	message := validMessage()
	message.Body = "before\n.\nafter\n"
	built, err := message.Build(sentAt)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if !strings.Contains(string(built), "\r\n..\r\n") {
		t.Fatalf("lone dot was not escaped:\n%q", built)
	}
}

// A non-ASCII subject must not reach the header as raw bytes.
func TestBuildEncodesNonASCIISubject(t *testing.T) {
	message := validMessage()
	message.Subject = "Gökçem sizi davet etti"
	built, err := message.Build(sentAt)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if !strings.Contains(string(built), "Subject: =?utf-8?q?") {
		t.Fatalf("subject was not encoded:\n%s", built)
	}
	if strings.Contains(string(built), "Subject: Gökçem") {
		t.Fatal("raw non-ASCII reached the subject header")
	}
}
