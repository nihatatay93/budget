package httpapi

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/nihatatay93/budget/internal/account"
	openapi "github.com/nihatatay93/budget/internal/api/openapi"
)

func (s *server) ListAccounts(
	ctx context.Context,
	request openapi.ListAccountsRequestObject,
) (openapi.ListAccountsResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	includeArchived := request.Params.IncludeArchived != nil && bool(*request.Params.IncludeArchived)
	values, err := s.Accounts.List(
		ctx, request.WorkspaceId.String(), principal.User.ID, includeArchived,
	)
	if err != nil {
		return nil, err
	}
	response := make([]openapi.Account, 0, len(values))
	for _, value := range values {
		converted, err := accountResponse(value)
		if err != nil {
			return nil, err
		}
		response = append(response, converted)
	}
	return openapi.ListAccounts200JSONResponse{
		Body:    openapi.AccountListResponse{Accounts: response},
		Headers: openapi.ListAccounts200ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func (s *server) CreateAccount(
	ctx context.Context,
	request openapi.CreateAccountRequestObject,
) (openapi.CreateAccountResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return openapi.CreateAccount400JSONResponse{BadRequestJSONResponse: badRequest(requestID)}, nil
	}
	value, err := s.Accounts.Create(
		ctx, request.WorkspaceId.String(), principal.User.ID, accountWriteInput(*request.Body),
	)
	if err != nil {
		return nil, err
	}
	converted, err := accountResponse(value)
	if err != nil {
		return nil, err
	}
	return openapi.CreateAccount201JSONResponse{
		Body: converted, Headers: openapi.CreateAccount201ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func (s *server) GetAccount(
	ctx context.Context,
	request openapi.GetAccountRequestObject,
) (openapi.GetAccountResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	value, err := s.Accounts.Get(
		ctx, request.WorkspaceId.String(), principal.User.ID, request.AccountId.String(),
	)
	if err != nil {
		return nil, err
	}
	converted, err := accountResponse(value)
	if err != nil {
		return nil, err
	}
	return openapi.GetAccount200JSONResponse{
		Body: converted, Headers: openapi.GetAccount200ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func (s *server) UpdateAccount(
	ctx context.Context,
	request openapi.UpdateAccountRequestObject,
) (openapi.UpdateAccountResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return openapi.UpdateAccount400JSONResponse{BadRequestJSONResponse: badRequest(requestID)}, nil
	}
	value, err := s.Accounts.Update(
		ctx, request.WorkspaceId.String(), principal.User.ID, request.AccountId.String(),
		accountWriteInput(*request.Body),
	)
	switch {
	case errors.Is(err, account.ErrInvalidInput):
		return openapi.UpdateAccount400JSONResponse{BadRequestJSONResponse: badRequest(requestID)}, nil
	case errors.Is(err, account.ErrCurrencyLocked):
		return openapi.UpdateAccount409JSONResponse{ConflictJSONResponse: conflict(
			requestID, "account_currency_locked", "Account currency cannot change after financial history exists.",
		)}, nil
	case err != nil:
		return nil, err
	}
	converted, err := accountResponse(value)
	if err != nil {
		return nil, err
	}
	return openapi.UpdateAccount200JSONResponse{
		Body: converted, Headers: openapi.UpdateAccount200ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func (s *server) ArchiveAccount(
	ctx context.Context,
	request openapi.ArchiveAccountRequestObject,
) (openapi.ArchiveAccountResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	err = s.Accounts.Archive(
		ctx, request.WorkspaceId.String(), principal.User.ID, request.AccountId.String(),
	)
	if err != nil {
		return nil, err
	}
	return openapi.ArchiveAccount204Response{
		Headers: openapi.ArchiveAccount204ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func accountWriteInput(input openapi.AccountWriteRequest) account.WriteInput {
	return account.WriteInput{
		Name: input.Name, Type: account.Type(input.Type), Currency: string(input.Currency),
		InstitutionName: input.InstitutionName,
	}
}

func accountResponse(value account.Account) (openapi.Account, error) {
	id, err := uuid.Parse(value.ID)
	if err != nil {
		return openapi.Account{}, err
	}
	workspaceID, err := uuid.Parse(value.WorkspaceID)
	if err != nil {
		return openapi.Account{}, err
	}
	return openapi.Account{
		Id: id, WorkspaceId: workspaceID, Name: value.Name, Type: openapi.AccountType(value.Type),
		Currency: openapi.Currency(value.Currency), InstitutionName: value.InstitutionName,
		ArchivedAt: value.ArchivedAt, BalanceMinor: value.BalanceMinor,
	}, nil
}
