package query

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/natalie-o-perret/go-functionalish/result"
)

// txer is the minimal interface that *sql.Tx satisfies, used internally.
type txer interface {
	Querier
	Commit() error
	Rollback() error
}

// Tx wraps *sql.Tx and implements [Querier].
// Obtain a Tx only via [WithTx]; do not construct directly.
type Tx struct {
	raw txer
}

// Querier returns the transaction as a [Querier] for use with all package-level
// query functions ([Query], [QueryAll], [QueryOne], [Exec]).
func (t *Tx) Querier() Querier { return t.raw }

// QueryContext implements [Querier].
func (t *Tx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.raw.QueryContext(ctx, query, args...)
}

// QueryRowContext implements [Querier].
func (t *Tx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.raw.QueryRowContext(ctx, query, args...)
}

// ExecContext implements [Querier].
func (t *Tx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.raw.ExecContext(ctx, query, args...)
}

// WithTx begins a transaction on db, calls fn with the wrapped [Tx], then:
//   - commits if fn returns Ok
//   - rolls back if fn returns Err (the original Err is returned unchanged)
//
// If the commit itself fails, the error is wrapped and returned as Err.
// The rollback error (if any) is silently discarded — the original fn error
// is always the one returned to the caller.
func WithTx[T any](
	ctx context.Context,
	db *DB,
	fn func(*Tx) result.Result[T, error],
) result.Result[T, error] {
	raw, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return result.Err[T, error](fmt.Errorf("tx: begin: %w", err))
	}
	tx := &Tx{raw: raw}
	r := fn(tx)
	if r.IsErr() {
		_ = raw.Rollback() // best-effort; original error takes priority
		return r
	}
	if err := raw.Commit(); err != nil {
		return result.Err[T, error](fmt.Errorf("tx: commit: %w", err))
	}
	return r
}
