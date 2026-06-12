package scan_test

import (
	"errors"
	"testing"

	"github.com/natalie-o-perret/go-sql/scan"
)

type testRow struct {
	ID   int64
	Name string
	Age  int
}

var testMapper = scan.NewMapper[testRow](
	scan.Column("id", func(r *testRow) *int64 { return &r.ID }),
	scan.Column("name", func(r *testRow) *string { return &r.Name }),
	scan.Column("age", func(r *testRow) *int { return &r.Age }),
)

func TestColumn_BuildsBinding(t *testing.T) {
	// Col[T] contains a function field and is not comparable via ==.
	// Verify it is usable: build a single-column mapper and scan successfully.
	col := scan.Column("id", func(r *testRow) *int64 { return &r.ID })
	m := scan.NewMapper[testRow](col)
	var row testRow
	if err := m.Scan([]string{"id"}, func(dest ...any) error {
		*dest[0].(*int64) = 99
		return nil
	}, &row); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row.ID != 99 {
		t.Fatalf("expected ID 99, got %d", row.ID)
	}
}
func TestMapper_Scan_AllColumns(t *testing.T) {
	cols := []string{"id", "name", "age"}
	calls := 0
	mockScan := func(dest ...any) error {
		calls++
		*dest[0].(*int64) = 1
		*dest[1].(*string) = "Alice"
		*dest[2].(*int) = 30
		return nil
	}
	var row testRow
	if err := testMapper.Scan(cols, mockScan, &row); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected scan called once, got %d", calls)
	}
	if row.ID != 1 || row.Name != "Alice" || row.Age != 30 {
		t.Fatalf("unexpected row: %+v", row)
	}
}
func TestMapper_Scan_UnknownColumnDiscarded(t *testing.T) {
	cols := []string{"id", "extra_col", "name", "age"}
	mockScan := func(dest ...any) error {
		*dest[0].(*int64) = 42
		// dest[1] is the discard pointer — write into it to confirm it does not panic
		*dest[1].(*any) = "ignored"
		*dest[2].(*string) = "Bob"
		*dest[3].(*int) = 25
		return nil
	}
	var row testRow
	if err := testMapper.Scan(cols, mockScan, &row); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row.ID != 42 || row.Name != "Bob" || row.Age != 25 {
		t.Fatalf("unexpected row: %+v", row)
	}
}
func TestMapper_Scan_PropagatesScanError(t *testing.T) {
	cols := []string{"id"}
	want := errors.New("driver error")
	mockScan := func(_ ...any) error { return want }
	var row testRow
	err := testMapper.Scan(cols, mockScan, &row)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, want) {
		t.Fatalf("expected wrapped driver error, got: %v", err)
	}
}
func TestMapper_Scan_PartialColumns(t *testing.T) {
	cols := []string{"name"}
	mockScan := func(dest ...any) error {
		*dest[0].(*string) = "Charlie"
		return nil
	}
	var row testRow
	if err := testMapper.Scan(cols, mockScan, &row); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row.Name != "Charlie" {
		t.Fatalf("unexpected name: %q", row.Name)
	}
	if row.ID != 0 || row.Age != 0 {
		t.Fatalf("expected zero values for unscanned fields, got: %+v", row)
	}
}
