package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	openapi "github.com/nihatatay93/budget/internal/api/openapi"
	"github.com/nihatatay93/budget/internal/auth"
)

const sessionCookieName = "budget_session"

type server struct {
	Services
	cookieSecure bool
	sessionTTL   time.Duration
}

var _ openapi.StrictServerInterface = (*server)(nil)

func (s *server) GetHealth(
	ctx context.Context,
	_ openapi.GetHealthRequestObject,
) (openapi.GetHealthResponseObject, error) {
	requestID := requestID(ctx)
	return openapi.GetHealth200JSONResponse{
		Body: openapi.HealthResponse{Status: "ok"},
		Headers: openapi.GetHealth200ResponseHeaders{
			XRequestID: &requestID,
		},
	}, nil
}

func (s *server) Register(
	ctx context.Context,
	request openapi.RegisterRequestObject,
) (openapi.RegisterResponseObject, error) {
	requestID := requestID(ctx)
	if request.Body == nil {
		return openapi.Register400JSONResponse{BadRequestJSONResponse: badRequest(requestID)}, nil
	}
	result, err := s.Authentication.Register(ctx, auth.RegisterInput{
		Email: string(request.Body.Email), Password: request.Body.Password,
		DisplayName: request.Body.DisplayName, WorkspaceName: request.Body.WorkspaceName,
		BaseCurrency: string(request.Body.BaseCurrency), Timezone: request.Body.Timezone,
		Transport: auth.Transport(request.Body.Transport),
	})
	if errors.Is(err, auth.ErrInvalidInput) {
		return openapi.Register400JSONResponse{BadRequestJSONResponse: badRequest(requestID)}, nil
	}
	if errors.Is(err, auth.ErrEmailTaken) {
		return openapi.Register409JSONResponse{ConflictJSONResponse: conflict(requestID, "email_taken", "That email address is already registered.")}, nil
	}
	if err != nil {
		return nil, err
	}
	body, err := authResponse(result)
	if err != nil {
		return nil, err
	}
	headers := openapi.Register201ResponseHeaders{XRequestID: &requestID}
	if result.Principal.Transport == auth.TransportCookie {
		cookie := s.sessionCookie(result.Token)
		headers.SetCookie = &cookie
		body.BearerToken = nil
	}
	return openapi.Register201JSONResponse{Body: body, Headers: headers}, nil
}

func (s *server) Login(
	ctx context.Context,
	request openapi.LoginRequestObject,
) (openapi.LoginResponseObject, error) {
	requestID := requestID(ctx)
	if request.Body == nil {
		return openapi.Login400JSONResponse{BadRequestJSONResponse: badRequest(requestID)}, nil
	}
	result, err := s.Authentication.Login(ctx, auth.LoginInput{
		Email: string(request.Body.Email), Password: request.Body.Password,
		Transport: auth.Transport(request.Body.Transport),
	})
	if errors.Is(err, auth.ErrInvalidCredentials) {
		return openapi.Login401JSONResponse{UnauthorizedJSONResponse: unauthorized(requestID)}, nil
	}
	if err != nil {
		return nil, err
	}
	body, err := authResponse(result)
	if err != nil {
		return nil, err
	}
	headers := openapi.Login200ResponseHeaders{XRequestID: &requestID}
	if result.Principal.Transport == auth.TransportCookie {
		cookie := s.sessionCookie(result.Token)
		headers.SetCookie = &cookie
		body.BearerToken = nil
	}
	return openapi.Login200JSONResponse{Body: body, Headers: headers}, nil
}

func (s *server) Logout(
	ctx context.Context,
	_ openapi.LogoutRequestObject,
) (openapi.LogoutResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.Authentication.Logout(ctx, principal); err != nil {
		return nil, err
	}
	headers := openapi.Logout204ResponseHeaders{XRequestID: &requestID}
	if principal.Transport == auth.TransportCookie {
		cookie := s.expiredSessionCookie()
		headers.SetCookie = &cookie
	}
	return openapi.Logout204Response{Headers: headers}, nil
}

func (s *server) GetSession(
	ctx context.Context,
	_ openapi.GetSessionRequestObject,
) (openapi.GetSessionResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.Authentication.Session(ctx, principal)
	if err != nil {
		return nil, err
	}
	body, err := sessionResponse(result)
	if err != nil {
		return nil, err
	}
	return openapi.GetSession200JSONResponse{
		Body: body, Headers: openapi.GetSession200ResponseHeaders{XRequestID: &requestID},
	}, nil
}

// sessionCookie carries the same lifetime as the session it names.
//
// Without Max-Age the browser discards it on close, so a web session ended at the whim of a
// window while the server still held it for the configured TTL — a lifetime the operator had
// set and the product then ignored. Expiring together means the cookie stops working when
// the session does, and not before.
func (s *server) sessionCookie(token string) string {
	return (&http.Cookie{
		Name: sessionCookieName, Value: token, Path: "/", HttpOnly: true,
		Secure: s.cookieSecure, SameSite: http.SameSiteLaxMode,
		MaxAge: int(s.sessionTTL.Seconds()),
	}).String()
}

func (s *server) expiredSessionCookie() string {
	return (&http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", HttpOnly: true,
		Secure: s.cookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1,
		Expires: time.Unix(1, 0).UTC(),
	}).String()
}

func authResponse(result auth.AuthResult) (openapi.AuthResponse, error) {
	session, err := sessionResponse(result)
	if err != nil {
		return openapi.AuthResponse{}, err
	}
	response := openapi.AuthResponse{User: session.User, Workspaces: session.Workspaces}
	if result.Principal.Transport == auth.TransportBearer {
		response.BearerToken = &result.Token
	}
	return response, nil
}

func sessionResponse(result auth.AuthResult) (openapi.SessionResponse, error) {
	userID, err := uuid.Parse(result.Principal.User.ID)
	if err != nil {
		return openapi.SessionResponse{}, err
	}
	workspaces := make([]openapi.WorkspaceSummary, 0, len(result.Workspaces))
	for _, workspace := range result.Workspaces {
		workspaceID, err := uuid.Parse(workspace.ID)
		if err != nil {
			return openapi.SessionResponse{}, err
		}
		workspaces = append(workspaces, openapi.WorkspaceSummary{
			Id: workspaceID, Name: workspace.Name, BaseCurrency: openapi.Currency(workspace.BaseCurrency),
			Timezone: workspace.Timezone, Role: openapi.WorkspaceRole(workspace.Role),
		})
	}
	return openapi.SessionResponse{
		User: openapi.User{
			Id: userID, Email: openapi_types.Email(result.Principal.User.Email),
			DisplayName: result.Principal.User.DisplayName,
		},
		Workspaces: workspaces,
	}, nil
}

func errorBody(requestID, code, message string) openapi.ErrorResponse {
	return openapi.ErrorResponse{Error: openapi.APIError{Code: code, Message: message, RequestId: requestID}}
}

func badRequest(requestID string) openapi.BadRequestJSONResponse {
	return openapi.BadRequestJSONResponse{
		Body:    errorBody(requestID, "invalid_request", "The request is invalid."),
		Headers: openapi.BadRequestResponseHeaders{XRequestID: &requestID},
	}
}

func unauthorized(requestID string) openapi.UnauthorizedJSONResponse {
	return openapi.UnauthorizedJSONResponse{
		Body:    errorBody(requestID, "unauthorized", "Authentication is required or the credentials are invalid."),
		Headers: openapi.UnauthorizedResponseHeaders{XRequestID: &requestID},
	}
}

func forbidden(requestID string) openapi.ForbiddenJSONResponse {
	return openapi.ForbiddenJSONResponse{
		Body:    errorBody(requestID, "forbidden", "You do not have access to this workspace operation."),
		Headers: openapi.ForbiddenResponseHeaders{XRequestID: &requestID},
	}
}

func notFound(requestID string) openapi.NotFoundJSONResponse {
	return openapi.NotFoundJSONResponse{
		Body:    errorBody(requestID, "not_found", "The requested resource was not found."),
		Headers: openapi.NotFoundResponseHeaders{XRequestID: &requestID},
	}
}

func gone(requestID, code, message string) openapi.GoneJSONResponse {
	return openapi.GoneJSONResponse{
		Body:    errorBody(requestID, code, message),
		Headers: openapi.GoneResponseHeaders{XRequestID: &requestID},
	}
}

func conflict(requestID, code, message string) openapi.ConflictJSONResponse {
	return openapi.ConflictJSONResponse{
		Body:    errorBody(requestID, code, message),
		Headers: openapi.ConflictResponseHeaders{XRequestID: &requestID},
	}
}

func serviceUnavailable(requestID, code, message string) openapi.ServiceUnavailableJSONResponse {
	return openapi.ServiceUnavailableJSONResponse{
		Body:    errorBody(requestID, code, message),
		Headers: openapi.ServiceUnavailableResponseHeaders{XRequestID: &requestID},
	}
}

func (s *server) GetReadiness(
	ctx context.Context,
	_ openapi.GetReadinessRequestObject,
) (openapi.GetReadinessResponseObject, error) {
	requestID := requestID(ctx)
	if err := s.Database.Ping(ctx); err != nil {
		return openapi.GetReadiness503JSONResponse{
			Body: openapi.HealthResponse{Status: "unavailable"},
			Headers: openapi.GetReadiness503ResponseHeaders{
				XRequestID: &requestID,
			},
		}, nil
	}
	return openapi.GetReadiness200JSONResponse{
		Body: openapi.HealthResponse{Status: "ready"},
		Headers: openapi.GetReadiness200ResponseHeaders{
			XRequestID: &requestID,
		},
	}, nil
}
