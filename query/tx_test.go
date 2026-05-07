package query_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/natalie-o-perret/go-functionalish/result"
	gosqlquery "github.com/natalie-o-perret/go-sql/query"
)

func TestWithTx_Commit(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	r := gosqlquery.WithTx(ctx, db, func(tx *gosqlquery.Tx) result.Result[int64, error] {
		er := gosqlquery.Exec(ctx, tx, `INSERT INTO products VALUES (50, "TxItem", 9.99)`)
		if er.IsErr() {
			return result.Err[int64, error](er.UnwrapErr())
		}
		id, _ := er.Unwrap().LastInsertId()
		return result.Ok[int64, error](id)
	})
	if r.IsErr() {
		t.Fatalf("unexpected error: %v", r.UnwrapErr())
	}
	if r.Unwrap() != 50 {
		t.Fatalf("expected last insert id 50, got %d", r.Unwrap())
	}
	// Verify the row was committed
	res := gosqlquery.QueryOne(ctx, db, productMapper, "SELECT id, label, price FROM products WHERE id = 50")
	if res.IsErr() || res.Unwrap().IsNone() {
		t.Fatal("expected committed row to be visible after WithTx")
	}
}
func TestWithTx_Rollback(t *testing.T) {
	db := newDB(t)
	seed(t, db)
	ctx := context.Background()
	customErr := fmt.Errorf("abort")
	r := gosqlquery.WithTx(ctx, db, func(tx *gosqlquery.Tx) result.Result[int64, error] {
		gosqlquery.Exec(ctx, tx, `DELETE FROM products`)
		return result.Err[int64, error](customErr)
	})
	if r.IsOk() {
		t.Fatal("expected Err after rollback, got Ok")
	}
	// All rows should still be present
	all := gosqlquery.QueryAll(ctx, db, productMapper, "SELECT id, label, price FROM products")
	if all.IsErr() {
		t.Fatalf("unexpected error: %v", all.UnwrapErr())
	}
	if len(all.Unwrap()) != 3 {
		t.Fatalf("expected 3 rows after rollback, got %d", len(all.Unwrap()))
	}
}
