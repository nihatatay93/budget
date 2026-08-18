package mail

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nihatatay93/budget/internal/workspace"
)

// InvitationNotifier turns an invitation into an email and sends it.
type InvitationNotifier struct {
	sender       *Sender
	fromAddress  string
	fromName     string
	publicOrigin string
}

func NewInvitationNotifier(
	sender *Sender, fromAddress, fromName, publicOrigin string,
) *InvitationNotifier {
	return &InvitationNotifier{
		sender: sender, fromAddress: fromAddress, fromName: fromName, publicOrigin: publicOrigin,
	}
}

// NotifyInvitation sends the acceptance token to the invited address.
//
// The token is the whole credential, so it goes in the body rather than a link query string:
// a link would put it in browser history, in any referrer the page later sends, and in the
// logs of anything that follows redirects. The recipient pastes it into the app instead.
func (n *InvitationNotifier) NotifyInvitation(
	_ context.Context, invitation workspace.Invitation, workspaceName, token string,
) error {
	subject := fmt.Sprintf("%s invited you to %s", invitation.InviterDisplayName, workspaceName)
	return n.sender.Send(Message{
		FromAddress: n.fromAddress,
		FromName:    n.fromName,
		To:          invitation.Email,
		Subject:     subject,
		Body:        n.body(invitation, workspaceName, token),
	})
}

func (n *InvitationNotifier) body(
	invitation workspace.Invitation, workspaceName, token string,
) string {
	var body strings.Builder
	fmt.Fprintf(&body, "%s invited you to join the %s workspace on Budget as %s.\n\n",
		invitation.InviterDisplayName, workspaceName, invitation.Role)
	body.WriteString("Sign in, open the workspace, and paste this invitation code:\n\n")
	fmt.Fprintf(&body, "    %s\n\n", token)
	if n.publicOrigin != "" {
		fmt.Fprintf(&body, "Budget is at %s\n\n", n.publicOrigin)
	}
	fmt.Fprintf(&body, "The code can be used once and expires on %s.\n",
		invitation.ExpiresAt.UTC().Format(time.RFC1123))
	body.WriteString("If you were not expecting this, you can ignore this message; " +
		"the code is useless without an account you control.\n")
	return body.String()
}
