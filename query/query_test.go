package query_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/natalie-o-perret/go-functionalish/seq"
	gosqlquery "github.com/natalie-o-perret/go-sql/query"
	"github.com/natalie-o-perret/go-sql/scan"
	_ "modernc.org/sqlite"
)

type product struct {
	ID    int64
	Label string
	Price float64
}

var productMapper = scan.NewMapper[product](
	scan.Column("id", func(p *product) *int64 { return &p.ID }),
	scan.Column("label", func(p *product) *string { return &p.Label }),
	scan.Column("price", func(p *product) *float64 { return &p.Price }),
)

func newDB(t *testing.T) *gosqlquery.DB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db := gosqlquery.New(sqlDB)
	ctx := context.Background()
	gosqlquery.Exec(ctx, db, `CREATE TABLE products (id INTEGER PRIMARY KEY, label TEXT, price REAL)`)
	return db
}
func seed(t *testing.T, db *gosqlquery.DB) {
	t.Helper()
	ctx := context.Background()
	gosqlquery.Exec(ctx, db, `INSERT INTO products VALUES (1, "Apple", 0.99), (2, "Banana", 0.49), (3, "Cherry", 2.99)`)
}

// -- QueryAll -----------------------------------------------------------------
func TestQueryAll_ReturnsAllRows(t *testing.T) {
	db := newDB(t)
	seed(t, db)
	ctx := context.Background()
	res := gosqlquery.QueryAll(ctx, db, productMapper, "SELECT id, label, price FROM products ORDER BY id")
	if res.IsErr() {
		t.Fatalf("unexpected error: %v", res.UnwrapErr())
	}
	rows := res.Unwrap()
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[0].Label != "Apple" || rows[1].Label != "Banana" || rows[2].Label != "Cherry" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}
func TestQueryAll_EmptyTable(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	res := gosqlquery.QueryAll(ctx, db, productMapper, "SELECT id, label, price FROM products")
	if res.IsErr() {
		t.Fatalf("unexpected error: %v", res.UnwrapErr())
	}
	if rows := res.Unwrap(); len(rows) != 0 {
		t.Fatalf("expected 0 rows, got %d", len(rows))
	}
}
func TestQueryAll_BadSQL_ReturnsErr(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	res := gosqlquery.QueryAll(ctx, db, productMapper, "SELECT * FROM nonexistent_table")
	if res.IsOk() {
		t.Fatal("expected Err for bad SQL, got Ok")
	}
}

// -- Query (lazy) -------------------------------------------------------------
func TestQuery_LazySeq(t *testing.T) {
	db := newDB(t)
	seed(t, db)
	ctx := context.Background()
	res := gosqlquery.Query(ctx, db, productMapper, "SELECT id, label, price FROM products ORDER BY id")
	if res.IsErr() {
		t.Fatalf("unexpected error: %v", res.UnwrapErr())
	}
	// seq.Map is package-level because Go cannot infer a new type param on a method
	labels := seq.Map(
		res.Unwrap().Filter(func(p product) bool { return p.Price > 1.0 }),
		func(p product) string { return p.Label },
	).ToSlice()
	if len(labels) != 1 || labels[0] != "Cherry" {
		t.Fatalf("unexpected labels: %v", labels)
	}
}

// -- QueryOne -----------------------------------------------------------------
func TestQueryOne_Found(t *testing.T) {
	db := newDB(t)
	seed(t, db)
	ctx := context.Background()
	res := gosqlquery.QueryOne(ctx, db, productMapper, "SELECT id, label, price FROM products WHERE id = ?", 2)
	if res.IsErr() {
		t.Fatalf("unexpected error: %v", res.UnwrapErr())
	}
	opt := res.Unwrap()
	if opt.IsNone() {
		t.Fatal("expected Some, got None")
	}
	p := opt.Unwrap()
	if p.Label != "Banana" {
		t.Fatalf("expected Banana, got %q", p.Label)
	}
}
func TestQueryOne_NotFound(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	res := gosqlquery.QueryOne(ctx, db, productMapper, "SELECT id, label, price FROM products WHERE id = ?", 999)
	if res.IsErr() {
		t.Fatalf("unexpected error: %v", res.UnwrapErr())
	}
	if res.Unwrap().IsSome() {
		t.Fatal("expected None, got Some")
	}
}

// -- Exec ---------------------------------------------------------------------
func TestExec_Insert(t *testing.T) {
	db := newDB(t)
	ctx := context.Background()
	r := gosqlquery.Exec(ctx, db, `INSERT INTO products VALUES (10, "Mango", 1.50)`)
	if r.IsErr() {
		t.Fatalf("unexpected error: %v", r.UnwrapErr())
	}
	n, _ := r.Unwrap().RowsAffected()
	if n != 1 {
		t.Fatalf("expected 1 row affected, got %d", n)
	}
}
