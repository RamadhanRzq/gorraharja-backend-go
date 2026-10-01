package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/database"
)

// rowQuerier is the subset of pgx used by repositories that accept an optional
// caller supplied transaction.
type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// pick resolves the connection a repository call should use: the transaction
// when the caller is already inside one, otherwise the pool. Keeping writes
// inside the caller's transaction is what makes multi-table mutations atomic.
func pick(db *database.DB, tx pgx.Tx) rowQuerier {
	if tx != nil {
		return tx
	}
	return db.Pool()
}
