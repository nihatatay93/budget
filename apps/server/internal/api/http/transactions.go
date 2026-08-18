package httpapi

import (
	"context"
	"errors"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	openapi "github.com/nihatatay93/budget/internal/api/openapi"
	"github.com/nihatatay93/budget/internal/transaction"
)

func (s *server) ListTransactions(
	ctx context.Context,
	request openapi.ListTransactionsRequestObject,
) (openapi.ListTransactionsResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	filter := transaction.ListFilter{}
	if request.Params.FromDate != nil {
		filter.From = &request.Params.FromDate.Time
	}
	if request.Params.ToDate != nil {
		filter.To = &request.Params.ToDate.Time
	}
	if request.Params.Limit != nil {
		filter.Limit = *request.Params.Limit
	}
	values, err := s.Transactions.List(
		ctx, request.WorkspaceId.String(), principal.User.ID, filter,
	)
	if err != nil {
		return nil, err
	}
	converted := make([]openapi.Transaction, 0, len(values))
	for _, value := range values {
		item, err := transactionResponse(value)
		if err != nil {
			return nil, err
		}
		converted = append(converted, item)
	}
	return openapi.ListTransactions200JSONResponse{
		Body:    openapi.TransactionListResponse{Transactions: converted},
		Headers: openapi.ListTransactions200ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func (s *server) CreateTransaction(
	ctx context.Context,
	request openapi.CreateTransactionRequestObject,
) (openapi.CreateTransactionResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return openapi.CreateTransaction400JSONResponse{BadRequestJSONResponse: badRequest(requestID)}, nil
	}
	value, err := s.Transactions.Create(
		ctx, request.WorkspaceId.String(), principal.User.ID, transactionWriteInput(*request.Body),
	)
	switch {
	case errors.Is(err, transaction.ErrInvalidInput):
		return openapi.CreateTransaction400JSONResponse{BadRequestJSONResponse: badRequest(requestID)}, nil
	case errors.Is(err, transaction.ErrDoesNotReconcile):
		return openapi.CreateTransaction409JSONResponse{ConflictJSONResponse: conflict(
			requestID, "transaction_does_not_reconcile", "Transaction entries and allocations do not reconcile.",
		)}, nil
	case errors.Is(err, transaction.ErrReferenceInvalid):
		return openapi.CreateTransaction409JSONResponse{ConflictJSONResponse: conflict(
			requestID, "transaction_reference_invalid", "A referenced account or category is unavailable.",
		)}, nil
	case errors.Is(err, transaction.ErrBookingRateUnavailable):
		return openapi.CreateTransaction503JSONResponse{ServiceUnavailableJSONResponse: serviceUnavailable(
			requestID, "booking_rate_unavailable", "A historical exchange rate is unavailable; provide an explicit base amount.",
		)}, nil
	case err != nil:
		return nil, err
	}
	converted, err := transactionResponse(value)
	if err != nil {
		return nil, err
	}
	return openapi.CreateTransaction201JSONResponse{
		Body: converted, Headers: openapi.CreateTransaction201ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func (s *server) GetTransaction(
	ctx context.Context,
	request openapi.GetTransactionRequestObject,
) (openapi.GetTransactionResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	value, err := s.Transactions.Get(
		ctx, request.WorkspaceId.String(), principal.User.ID, request.TransactionId.String(),
	)
	if err != nil {
		return nil, err
	}
	converted, err := transactionResponse(value)
	if err != nil {
		return nil, err
	}
	return openapi.GetTransaction200JSONResponse{
		Body: converted, Headers: openapi.GetTransaction200ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func (s *server) UpdateTransaction(
	ctx context.Context,
	request openapi.UpdateTransactionRequestObject,
) (openapi.UpdateTransactionResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return openapi.UpdateTransaction400JSONResponse{BadRequestJSONResponse: badRequest(requestID)}, nil
	}
	value, err := s.Transactions.Update(
		ctx, request.WorkspaceId.String(), principal.User.ID, request.TransactionId.String(),
		transactionWriteInput(*request.Body),
	)
	switch {
	case errors.Is(err, transaction.ErrInvalidInput):
		return openapi.UpdateTransaction400JSONResponse{BadRequestJSONResponse: badRequest(requestID)}, nil
	case errors.Is(err, transaction.ErrDoesNotReconcile):
		return openapi.UpdateTransaction409JSONResponse{ConflictJSONResponse: conflict(
			requestID, "transaction_does_not_reconcile", "Transaction entries and allocations do not reconcile.",
		)}, nil
	case errors.Is(err, transaction.ErrReferenceInvalid):
		return openapi.UpdateTransaction409JSONResponse{ConflictJSONResponse: conflict(
			requestID, "transaction_reference_invalid", "A referenced account or category is unavailable.",
		)}, nil
	case errors.Is(err, transaction.ErrBookingRateUnavailable):
		return openapi.UpdateTransaction503JSONResponse{ServiceUnavailableJSONResponse: serviceUnavailable(
			requestID, "booking_rate_unavailable", "A historical exchange rate is unavailable; provide an explicit base amount.",
		)}, nil
	case err != nil:
		return nil, err
	}
	converted, err := transactionResponse(value)
	if err != nil {
		return nil, err
	}
	return openapi.UpdateTransaction200JSONResponse{
		Body: converted, Headers: openapi.UpdateTransaction200ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func (s *server) DeleteTransaction(
	ctx context.Context,
	request openapi.DeleteTransactionRequestObject,
) (openapi.DeleteTransactionResponseObject, error) {
	requestID := requestID(ctx)
	principal, err := requirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	err = s.Transactions.SoftDelete(
		ctx, request.WorkspaceId.String(), principal.User.ID, request.TransactionId.String(),
	)
	if err != nil {
		return nil, err
	}
	return openapi.DeleteTransaction204Response{
		Headers: openapi.DeleteTransaction204ResponseHeaders{XRequestID: &requestID},
	}, nil
}

func transactionWriteInput(input openapi.TransactionWriteRequest) transaction.WriteInput {
	entries := make([]transaction.EntryInput, 0, len(input.Entries))
	for _, entry := range input.Entries {
		entries = append(entries, transaction.EntryInput{
			AccountID: entry.AccountId.String(), AmountMinor: entry.AmountMinor,
			BaseAmountMinor: entry.BaseAmountMinor,
		})
	}
	allocations := make([]transaction.AllocationInput, 0)
	if input.Allocations != nil {
		allocations = make([]transaction.AllocationInput, 0, len(*input.Allocations))
		for _, allocation := range *input.Allocations {
			allocations = append(allocations, transaction.AllocationInput{
				CategoryID: allocation.CategoryId.String(), AmountBaseMinor: allocation.AmountBaseMinor,
			})
		}
	}
	return transaction.WriteInput{
		Kind: transaction.Kind(input.Kind), Status: transaction.Status(input.Status),
		TransactionDate: input.TransactionDate.Time, Payee: input.Payee,
		Description: input.Description, Notes: input.Notes,
		Entries: entries, Allocations: allocations,
	}
}

func transactionResponse(value transaction.Transaction) (openapi.Transaction, error) {
	id, err := uuid.Parse(value.ID)
	if err != nil {
		return openapi.Transaction{}, err
	}
	workspaceID, err := uuid.Parse(value.WorkspaceID)
	if err != nil {
		return openapi.Transaction{}, err
	}
	createdBy, err := uuid.Parse(value.CreatedBy)
	if err != nil {
		return openapi.Transaction{}, err
	}
	updatedBy, err := uuid.Parse(value.UpdatedBy)
	if err != nil {
		return openapi.Transaction{}, err
	}
	entries := make([]openapi.TransactionEntry, 0, len(value.Entries))
	for _, entry := range value.Entries {
		accountID, err := uuid.Parse(entry.AccountID)
		if err != nil {
			return openapi.Transaction{}, err
		}
		entries = append(entries, openapi.TransactionEntry{
			AccountId: accountID, AmountMinor: entry.AmountMinor,
			BaseAmountMinor: entry.BaseAmountMinor,
		})
	}
	allocations := make([]openapi.TransactionAllocation, 0, len(value.Allocations))
	for _, allocation := range value.Allocations {
		categoryID, err := uuid.Parse(allocation.CategoryID)
		if err != nil {
			return openapi.Transaction{}, err
		}
		allocations = append(allocations, openapi.TransactionAllocation{
			CategoryId: categoryID, AmountBaseMinor: allocation.AmountBaseMinor,
		})
	}
	return openapi.Transaction{
		Id: id, WorkspaceId: workspaceID, Kind: openapi.TransactionKind(value.Kind),
		Status:          openapi.TransactionStatus(value.Status),
		TransactionDate: openapi_types.Date{Time: value.TransactionDate},
		Payee:           value.Payee, Description: value.Description, Notes: value.Notes,
		Source: openapi.TransactionSource(value.Source), CreatedBy: createdBy, UpdatedBy: updatedBy,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Entries: entries, Allocations: allocations,
	}, nil
}
