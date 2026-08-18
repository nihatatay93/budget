package workspace

import "context"

// InvitationNotifier delivers an invitation to its recipient.
//
// The token is passed separately from the invitation because it is a credential that exists
// only in memory: the repository stores a hash, so this is the one opportunity to send it.
type InvitationNotifier interface {
	NotifyInvitation(ctx context.Context, invitation Invitation, workspaceName, token string) error
}

// WorkspaceNameLookup resolves the name shown in an invitation message.
type WorkspaceNameLookup interface {
	WorkspaceName(ctx context.Context, workspaceID string) (string, error)
}
