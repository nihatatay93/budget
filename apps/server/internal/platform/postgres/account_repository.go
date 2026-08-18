package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nihatatay93/budget/internal/account"
	"github.com/nihatatay93/budget/internal/platform/postgres/sqlc"
)

type AccountRepository struct {
	pool *pgxpool.Pool
}

func NewAccountRepository(pool *pgxpool.Pool) *AccountRepository {
	return &AccountRepository{pool: pool}
}

func (r *AccountRepository) List(
	ctx context.Context,
	workspaceID string,
	includeArchived bool,
) ([]account.Account, error) {
	workspaceUUID, err := postgresUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	rows, err := sqlc.New(r.pool).ListAccounts(ctx, sqlc.ListAccountsParams{
		WorkspaceID: workspaceUUID, IncludeArchived: includeArchived,
	})
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	accounts := make([]account.Account, 0, len(rows))
	for _, row := range rows {
		value, err := accountFromDatabase(
			row.ID, row.WorkspaceID, row.Name, row.Type, row.Currency,
			row.InstitutionName, row.ArchivedAt, row.BalanceMinor,
		)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, value)
	}
	return accounts, nil
}

func (r *AccountRepository) Get(
	ctx context.Context,
	workspaceID, accountID string,
) (account.Account, error) {
	workspaceUUID, accountUUID, err := resourceUUIDs(workspaceID, accountID)
	if err != nil {
		return account.Account{}, err
	}
	row, err := sqlc.New(r.pool).GetAccount(ctx, sqlc.GetAccountParams{
		WorkspaceID: workspaceUUID, ID: accountUUID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return account.Account{}, account.ErrNotFound
	}
	if err != nil {
		return account.Account{}, fmt.Errorf("get account: %w", err)
	}
	return accountFromDatabase(
		row.ID, row.WorkspaceID, row.Name, row.Type, row.Currency,
		row.InstitutionName, row.ArchivedAt, row.BalanceMinor,
	)
}

func (r *AccountRepository) Create(ctx context.Context, value account.Account) (account.Account, error) {
	workspaceUUID, accountUUID, err := resourceUUIDs(value.WorkspaceID, value.ID)
	if err != nil {
		return account.Account{}, err
	}
	if err := sqlc.New(r.pool).CreateAccount(ctx, sqlc.CreateAccountParams{
		ID: accountUUID, WorkspaceID: workspaceUUID, Name: value.Name,
		Type: string(value.Type), Currency: value.Currency,
		InstitutionName: postgresText(value.InstitutionName),
	}); err != nil {
		return account.Account{}, fmt.Errorf("create account: %w", err)
	}
	return r.Get(ctx, value.WorkspaceID, value.ID)
}

func (r *AccountRepository) Update(
	ctx context.Context,
	workspaceID, accountID string,
	input account.WriteInput,
) (account.Account, error) {
	workspaceUUID, accountUUID, err := resourceUUIDs(workspaceID, accountID)
	if err != nil {
		return account.Account{}, err
	}
	rows, err := sqlc.New(r.pool).UpdateAccount(ctx, sqlc.UpdateAccountParams{
		Name: input.Name, Type: string(input.Type), Currency: input.Currency,
		InstitutionName: postgresText(input.InstitutionName),
		WorkspaceID:     workspaceUUID, ID: accountUUID,
	})
	if constraintViolation(err, "accounts_currency_with_history") {
		return account.Account{}, account.ErrCurrencyLocked
	}
	if err != nil {
		return account.Account{}, fmt.Errorf("update account: %w", err)
	}
	if rows == 0 {
		return account.Account{}, account.ErrNotFound
	}
	return r.Get(ctx, workspaceID, accountID)
}

func (r *AccountRepository) Archive(ctx context.Context, workspaceID, accountID string) error {
	workspaceUUID, accountUUID, err := resourceUUIDs(workspaceID, accountID)
	if err != nil {
		return err
	}
	rows, err := sqlc.New(r.pool).ArchiveAccount(ctx, sqlc.ArchiveAccountParams{
		WorkspaceID: workspaceUUID, ID: accountUUID,
	})
	if err != nil {
		return fmt.Errorf("archive account: %w", err)
	}
	if rows == 0 {
		return account.ErrNotFound
	}
	return nil
}

func accountFromDatabase(
	id, workspaceID pgtype.UUID,
	name, accountType, currency string,
	institution pgtype.Text,
	archived pgtype.Timestamptz,
	balance int64,
) (account.Account, error) {
	idString, err := stringUUID(id)
	if err != nil {
		return account.Account{}, err
	}
	workspaceIDString, err := stringUUID(workspaceID)
	if err != nil {
		return account.Account{}, err
	}
	return account.Account{
		ID: idString, WorkspaceID: workspaceIDString, Name: name, Type: account.Type(accountType),
		Currency: currency, InstitutionName: stringPointer(institution),
		ArchivedAt: timePointer(archived), BalanceMinor: balance,
	}, nil
}

func resourceUUIDs(workspaceID, resourceID string) (pgtype.UUID, pgtype.UUID, error) {
	workspaceUUID, err := postgresUUID(workspaceID)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, err
	}
	resourceUUID, err := postgresUUID(resourceID)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, err
	}
	return workspaceUUID, resourceUUID, nil
}

func postgresText(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}

func stringPointer(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	result := value.String
	return &result
}

func timePointer(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func constraintViolation(err error, constraint string) bool {
	var databaseError *pgconn.PgError
	return errors.As(err, &databaseError) && databaseError.ConstraintName == constraint
}
