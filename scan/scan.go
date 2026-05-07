// Package scan provides zero-reflection, generics-native column -> field mapping
// for database/sql result sets.
//
// Mappings are registered once as explicit typed [Col] bindings -- no struct
// tags, no reflection, no code generation. At scan time, [Mapper.Scan] builds
// a correctly-ordered slice of scan targets from the live column list returned
// by rows.Columns() and delegates directly to rows.Scan, exactly as hand-written.
//
// Unknown columns are silently discarded into a throwaway pointer.
//
// # Example
//
//	type User struct { ID int64; Name string; Age int }
//
//	var UserMapper = scan.NewMapper[User](
//	    scan.Column("id",   func(u *User) *int64  { return &u.ID }),
//	    scan.Column("name", func(u *User) *string { return &u.Name }),
//	    scan.Column("age",  func(u *User) *int    { return &u.Age }),
//	)
package scan

import "fmt"

// colBinding holds the column name and a lazily-evaluated pointer factory.
// The factory is called with the destination struct pointer at scan time,
// returning the correctly-typed *V cast to any so rows.Scan can receive it.
type colBinding[T any] struct {
	name       string
	scanTarget func(dest *T) any
}

// Col[T] is a fully-constructed column binding for type T.
// Use [Column] to build one.
type Col[T any] struct {
	b colBinding[T]
}

// Column creates a binding that scans the named SQL column directly into the
// field returned by getter -- no intermediate allocation, no reflection.
//
//	scan.Column("id", func(u *User) *int64 { return &u.ID })
func Column[T any, V any](name string, getter func(*T) *V) Col[T] {
	return Col[T]{
		b: colBinding[T]{
			name: name,
			scanTarget: func(dest *T) any {
				return getter(dest)
			},
		},
	}
}

// Mapper[T] holds all column -> field bindings for type T.
// Build one via [NewMapper]; it is safe to reuse across goroutines.
type Mapper[T any] struct {
	bindings map[string]colBinding[T]
}

// NewMapper assembles a Mapper[T] from the provided [Col] bindings.
// If two bindings share the same column name, the last one wins.
func NewMapper[T any](cols ...Col[T]) Mapper[T] {
	m := Mapper[T]{bindings: make(map[string]colBinding[T], len(cols))}
	for _, c := range cols {
		m.bindings[c.b.name] = c.b
	}
	return m
}

// Scan populates *dest by scanning the columns listed in cols.
// scan must be rows.Scan (or a compatible function).
// Unknown columns are silently discarded.
func (m Mapper[T]) Scan(cols []string, scan func(dest ...any) error, dest *T) error {
	targets := make([]any, len(cols))
	for i, col := range cols {
		if b, ok := m.bindings[col]; ok {
			targets[i] = b.scanTarget(dest)
		} else {
			targets[i] = new(any) // discard unknown column
		}
	}
	if err := scan(targets...); err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	return nil
}
