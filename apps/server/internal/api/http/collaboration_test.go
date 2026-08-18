package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nihatatay93/budget/internal/auth"
	"github.com/nihatatay93/budget/internal/workspace"
)

const (
	testMemberID     = "0198b7ae-5e93-72d9-b47a-54e7634ee5a1"
	testInvitationID = "0198b7ae-5e93-72da-8ac4-d8155299b48a"
)

type fakeCollaborationService struct {
	members          []workspace.Member
	invitations      []workspace.Invitation
	credential       workspace.InvitationCredential
	acceptance       workspace.Acceptance
	err              error
	workspaceID      string
	actorID          string
	targetID         string
	invitationID     string
	invitationInput  workspace.InvitationInput
	acceptedToken    string
	createCalls      int
	acceptCalls      int
	memberWriteCalls int
}

func (s *fakeCollaborationService) ListMembers(
	_ context.Context, workspaceID, actorID string,
) ([]workspace.Member, error) {
	s.workspaceID, s.actorID = workspaceID, actorID
	return s.members, s.err
}

func (s *fakeCollaborationService) ListInvitations(
	context.Context, string, string,
) ([]workspace.Invitation, error) {
	return s.invitations, s.err
}

func (s *fakeCollaborationService) CreateInvitation(
	_ context.Context, workspaceID, actorID string, input workspace.InvitationInput,
) (workspace.InvitationCredential, error) {
	s.workspaceID, s.actorID, s.invitationInput = workspaceID, actorID, input
	s.createCalls++
	return s.credential, s.err
}

func (s *fakeCollaborationService) RevokeInvitation(
	_ context.Context, workspaceID, actorID, invitationID string,
) error {
	s.workspaceID, s.actorID, s.invitationID = workspaceID, actorID, invitationID
	s.memberWriteCalls++
	return s.err
}

func (s *fakeCollaborationService) UpdateMemberRole(
	_ context.Context, workspaceID, actorID, targetID string, role workspace.Role,
) (workspace.Member, error) {
	s.workspaceID, s.actorID, s.targetID = workspaceID, actorID, targetID
	s.memberWriteCalls++
	return workspace.Member{UserID: targetID, Role: role, JoinedAt: time.Now()}, s.err
}

func (s *fakeCollaborationService) RemoveMember(
	_ context.Context, workspaceID, actorID, targetID string,
) error {
	s.workspaceID, s.actorID, s.targetID = workspaceID, actorID, targetID
	s.memberWriteCalls++
	return s.err
}

func (s *fakeCollaborationService) AcceptInvitation(
	_ context.Context, actorID, token string,
) (workspace.Acceptance, error) {
	s.actorID, s.acceptedToken = actorID, token
	s.acceptCalls++
	return s.acceptance, s.err
}

func TestCreateWorkspaceInvitationReturnsOneTimeCredential(t *testing.T) {
	now := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	token := strings.Repeat("a", 43)
	service := &fakeCollaborationService{credential: workspace.InvitationCredential{
		Invitation: workspace.Invitation{
			ID: testInvitationID, WorkspaceID: testWorkspaceID,
			Email: "person@example.com", Role: workspace.RoleMember,
			InvitedBy: testUserID, InviterDisplayName: "Owner",
			ExpiresAt: now.Add(workspace.InvitationLifetime), CreatedAt: now,
		},
		Token: token,
	}}
	response := performJSON(
		t, collaborationTestRouter(t, service, "bearer"), http.MethodPost,
		"/v1/workspaces/"+testWorkspaceID+"/invitations",
		`{"email":"person@example.com","role":"member"}`,
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if service.workspaceID != testWorkspaceID || service.actorID != testUserID ||
		service.invitationInput.Email != "person@example.com" ||
		service.invitationInput.Role != workspace.RoleMember {
		t.Fatalf("CreateInvitation() call = %#v", service)
	}
	var body struct {
		AcceptanceToken string `json:"acceptance_token"`
		Invitation      struct {
			ID string `json:"id"`
		} `json:"invitation"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.AcceptanceToken != token || body.Invitation.ID != testInvitationID {
		t.Fatalf("create response = %#v", body)
	}
}

func TestAcceptWorkspaceInvitationUsesBodyToken(t *testing.T) {
	token := strings.Repeat("b", 43)
	now := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	service := &fakeCollaborationService{acceptance: workspace.Acceptance{
		Workspace: workspace.WorkspaceSummary{
			ID: testWorkspaceID, Name: "Family", BaseCurrency: "TRY",
			Timezone: "Europe/Istanbul", Role: workspace.RoleMember,
		},
		Member: workspace.Member{
			UserID: testUserID, Email: "person@example.com", DisplayName: "Person",
			Role: workspace.RoleMember, JoinedAt: now,
		},
	}}
	response := performJSON(
		t, collaborationTestRouter(t, service, "bearer"), http.MethodPost,
		"/v1/invitations/accept", `{"token":"`+token+`"}`,
		map[string]string{"Authorization": "Bearer raw-token"},
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if service.acceptedToken != token || service.actorID != testUserID || service.acceptCalls != 1 {
		t.Fatalf("AcceptInvitation() token/call = %q/%d", service.acceptedToken, service.acceptCalls)
	}
}

func TestCollaborationErrorMapping(t *testing.T) {
	tests := []struct {
		name, method, path, body string
		err                      error
		wantStatus               int
		wantCode                 string
	}{
		{
			name: "last owner role", method: http.MethodPatch,
			path: "/v1/workspaces/" + testWorkspaceID + "/members/" + testUserID,
			body: `{"role":"member"}`, err: workspace.ErrLastOwner,
			wantStatus: http.StatusConflict, wantCode: "last_owner",
		},
		{
			name: "already member", method: http.MethodPost, path: "/v1/invitations/accept",
			body: `{"token":"` + strings.Repeat("c", 43) + `"}`, err: workspace.ErrAlreadyMember,
			wantStatus: http.StatusConflict, wantCode: "already_member",
		},
		{
			name: "expired invitation", method: http.MethodPost, path: "/v1/invitations/accept",
			body: `{"token":"` + strings.Repeat("d", 43) + `"}`, err: workspace.ErrInvitationUnavailable,
			wantStatus: http.StatusGone, wantCode: "invitation_unavailable",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeCollaborationService{err: test.err}
			response := performJSON(
				t, collaborationTestRouter(t, service, "bearer"), test.method, test.path, test.body,
				map[string]string{"Authorization": "Bearer raw-token"},
			)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.wantStatus, response.Body.String())
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Error.Code != test.wantCode {
				t.Fatalf("error code = %q, want %q", body.Error.Code, test.wantCode)
			}
		})
	}
}

func TestCookieAuthenticationRejectsCrossSiteInvitationCreation(t *testing.T) {
	service := &fakeCollaborationService{}
	response := performJSON(
		t, collaborationTestRouter(t, service, "cookie"), http.MethodPost,
		"/v1/workspaces/"+testWorkspaceID+"/invitations",
		`{"email":"person@example.com","role":"member"}`,
		map[string]string{
			"Cookie": "budget_session=raw-token", "Origin": "https://attacker.example",
			"Sec-Fetch-Site": "cross-site",
		},
	)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if service.createCalls != 0 {
		t.Fatal("cross-site request reached collaboration service")
	}
}

func collaborationTestRouter(
	t *testing.T, collaboration collaborationService, transport auth.Transport,
) http.Handler {
	t.Helper()
	services := testServices()
	services.Authentication = &fakeAuthService{principal: testAuthResult(transport).Principal}
	services.Collaboration = collaboration
	return testRouter(t, services)
}
