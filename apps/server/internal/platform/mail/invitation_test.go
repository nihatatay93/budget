package mail

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nihatatay93/budget/internal/workspace"
)

func testInvitation() workspace.Invitation {
	return workspace.Invitation{
		ID:                 "0198b7ae-5e93-72d9-ab00-32b0861a3f37",
		WorkspaceID:        "0198b7ae-5e93-72d8-99af-ff40c48ad342",
		Email:              "invited@example.com",
		Role:               workspace.RoleMember,
		InviterDisplayName: "Owner",
		ExpiresAt:          time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
	}
}

// The message must carry the code, name the workspace and role, and say when it expires --
// enough for the recipient to act without a second message.
func TestInvitationMessageCarriesTheCodeAndContext(t *testing.T) {
	notifier := NewInvitationNotifier(nil, "budget@example.com", "Budget", "https://budget.example")
	body := notifier.body(testInvitation(), "Atay Family", "a-single-use-token")

	for _, expected := range []string{
		"Owner", "Atay Family", "member", "a-single-use-token",
		"https://budget.example", "expires",
	} {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(expected)) {
			t.Fatalf("body is missing %q:\n%s", expected, body)
		}
	}
}

// The token is the entire credential. Putting it in a link would place it in browser history
// and in the referrer of any page the recipient visits next, so it must not appear as a URL.
func TestInvitationMessageDoesNotPutTheTokenInALink(t *testing.T) {
	notifier := NewInvitationNotifier(nil, "budget@example.com", "Budget", "https://budget.example")
	const token = "a-single-use-token"
	body := notifier.body(testInvitation(), "Atay Family", token)

	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, token) && strings.Contains(line, "://") {
			t.Fatalf("the token appears inside a URL: %q", line)
		}
	}
	if strings.Contains(body, "?token=") || strings.Contains(body, "/accept/"+token) {
		t.Fatalf("the token was embedded in a link:\n%s", body)
	}
}

// An inviter display name is user-controlled and reaches the subject, so the notifier must
// refuse rather than emit a message with attacker-chosen headers.
func TestInvitationRefusesAnInjectedDisplayName(t *testing.T) {
	notifier := NewInvitationNotifier(
		NewSender(Options{Host: "localhost", Port: 587, Timeout: time.Second}),
		"budget@example.com", "Budget", "",
	)
	invitation := testInvitation()
	invitation.InviterDisplayName = "Owner\r\nBcc: attacker@example.com"

	err := notifier.NotifyInvitation(context.Background(), invitation, "Atay Family", "token")
	if err == nil {
		t.Fatal("NotifyInvitation() accepted an injected display name")
	}
}
