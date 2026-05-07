// Package query provides zero-reflection, generics-native SQL query execution
// over database/sql. It pairs naturally with the scan package for column
// mapping and with go-functionalish for railway-oriented result handling.
//
// All functions accept a [Querier] — satisfied by both *sql.DB and the [Tx]
// type returned by [WithTx] — so the same code runs inside or outside a
// transaction without changes.
//
// # Return types
//
//   - [Query] returns result.Result[seq.Seq[T], error]: a lazy cursor-backed Seq.
//   - [QueryAll] returns result.Result[[]T, error]: fully-materialised with full error propagation.
//   - [QueryOne] returns result.Result[option.Option[T], error]: first row or None.
//   - [Exec] returns result.Result[sql.Result, error]: for INSERT / UPDATE / DELETE / DDL.
//
// # Example
//
// db := query.New(sqlDB)
// res := query.QueryAll(ctx, db, UserMapper, "SELECT id, name, age FROM users WHERE age > $1", 18)
// users := res.UnwrapOr(nil)
package query

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/natalie-o-perret/go-functionalish/option"
	"github.com/natalie-o-perret/go-functionalish/result"
	"github.com/natalie-o-perret/go-functionalish/seq"
	"github.com/natalie-o-perret/go-sql/scan"
)

// Querier is the minimal interface satisfied by both *sql.DB and [Tx].
// It enables query functions to be called inside or outside a transaction
// without any API change.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// DB wraps *sql.DB and implements [Querier].
// Obtain one via [New].
type DB struct {
	db *sql.DB
}

// New wraps a *sql.DB.
func New(db *sql.DB) *DB { return &DB{db: db} }

// Raw returns the underlying *sql.DB for escape-hatch access (e.g. BeginTx options).
func (d *DB) Raw() *sql.DB { return d.db }

// QueryContext implements [Querier].
func (d *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return d.db.QueryContext(ctx, query, args...)
}

// QueryRowContext implements [Querier].
func (d *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return d.db.QueryRowContext(ctx, query, args...)
}

// ExecContext implements [Querier].
func (d *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.db.ExecContext(ctx, query, args...)
}

// ---------------------------------------------------------------------------
// Query — lazy cursor
// ---------------------------------------------------------------------------
// Query executes a SELECT and returns a lazy [seq.Seq[T]] backed by the live
// *sql.Rows cursor. Rows are closed when the Seq is fully iterated or when the
// consumer breaks early. Scan errors during iteration terminate the Seq silently;
// use [QueryAll] when per-row error propagation is required.
func Query[T any](
	ctx context.Context,
	q Querier,
	m scan.Mapper[T],
	query string,
	args ...any,
) result.Result[seq.Seq[T], error] {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return result.Err[seq.Seq[T], error](fmt.Errorf("query: %w", err))
	}
	cols, err := rows.Columns()
	if err != nil {
		_ = rows.Close()
		return result.Err[seq.Seq[T], error](fmt.Errorf("query: columns: %w", err))
	}
	s := seq.Seq[T](func(yield func(T) bool) {
		defer rows.Close() //nolint:errcheck
		for rows.Next() {
			var t T
			if err := m.Scan(cols, rows.Scan, &t); err != nil {
				return
			}
			if !yield(t) {
				return
			}
		}
	})
	return result.Ok[seq.Seq[T], error](s)
}

// ---------------------------------------------------------------------------
// QueryAll — eager, full error propagation
// ---------------------------------------------------------------------------
// QueryAll executes a SELECT and materialises all rows into a []T.
// Unlike [Query], scan errors and rows.Err() are both propagated as Err.
func QueryAll[T any](
	ctx context.Context,
	q Querier,
	m scan.Mapper[T],
	query string,
	args ...any,
) result.Result[[]T, error] {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return result.Err[[]T, error](fmt.Errorf("query: %w", err))
	}
	defer rows.Close() //nolint:errcheck
	cols, err := rows.Columns()
	if err != nil {
		return result.Err[[]T, error](fmt.Errorf("query: columns: %w", err))
	}
	var out []T
	for rows.Next() {
		var t T
		if err := m.Scan(cols, rows.Scan, &t); err != nil {
			return result.Err[[]T, error](err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return result.Err[[]T, error](fmt.Errorf("query: rows: %w", err))
	}
	return result.Ok[[]T, error](out)
}

// ---------------------------------------------------------------------------
// QueryOne — single row as Option
// ---------------------------------------------------------------------------
// QueryOne executes a SELECT and returns the first row wrapped in
// [option.Option[T]]. If no row is returned the result is Ok(None).
// If more than one row is present only the first is consumed.
func QueryOne[T any](
	ctx context.Context,
	q Querier,
	m scan.Mapper[T],
	query string,
	args ...any,
) result.Result[option.Option[T], error] {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return result.Err[option.Option[T], error](fmt.Errorf("query one: %w", err))
	}
	defer rows.Close() //nolint:errcheck
	cols, err := rows.Columns()
	if err != nil {
		return result.Err[option.Option[T], error](fmt.Errorf("query one: columns: %w", err))
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return result.Err[option.Option[T], error](fmt.Errorf("query one: rows: %w", err))
		}
		return result.Ok[option.Option[T], error](option.None[T]())
	}
	var t T
	if err := m.Scan(cols, rows.Scan, &t); err != nil {
		return result.Err[option.Option[T], error](err)
	}
	return result.Ok[option.Option[T], error](option.Some(t))
}

// ---------------------------------------------------------------------------
// Exec — no result rows
// ---------------------------------------------------------------------------
// Exec executes a statement that returns no rows (INSERT, UPDATE, DELETE, DDL)
// and returns the [sql.Result] wrapped in a [result.Result].
func Exec(
	ctx context.Context,
	q Querier,
	query string,
	args ...any,
) result.Result[sql.Result, error] {
	r, err := q.ExecContext(ctx, query, args...)
	if err != nil {
		return result.Err[sql.Result, error](fmt.Errorf("exec: %w", err))
	}
	return result.Ok[sql.Result, error](r)
}
