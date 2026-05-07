# go-sql

[![CI](https://github.com/natalie-o-perret/go-sql/actions/workflows/ci.yml/badge.svg)](https://github.com/natalie-o-perret/go-sql/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/natalie-o-perret/go-sql.svg)](https://pkg.go.dev/github.com/natalie-o-perret/go-sql)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Contributing](https://img.shields.io/badge/contributing-guide-blue)](CONTRIBUTING.md)
A zero-reflection, generics-native SQL library for Go 1.24+.
No reflection. No struct tags. No code generation. Explicit column bindings and railway-oriented error handling.
> [!NOTE]
> Companion library to [go-functionalish](https://github.com/natalie-o-perret/go-functionalish).
>
> Query results are `result.Result[seq.Seq[T], error]` or `result.Result[option.Option[T], error]` —
> first-class citizens in a railway pipeline, not bare `(T, error)` pairs with sixty `if err != nil` blocks.

## Packages

| Package | Description                                                                            |
|---------|----------------------------------------------------------------------------------------|
| `scan`  | `Mapper[T]` -- explicit, zero-reflection column -> field bindings                      |
| `query` | `Query[T]`, `QueryAll[T]`, `QueryOne[T]`, `Exec`, `WithTx[T]` -- typed query execution |

## Quick start

```go
import (
    "github.com/natalie-o-perret/go-sql/scan"
    "github.com/natalie-o-perret/go-sql/query"
)
```

### scan: explicit column bindings

Define a mapper once, reuse it everywhere. No reflection, no struct tags, no magic.

```go
type User struct {
    ID   int64
    Name string
    Age  int
}
// Explicit getter func -- the compiler verifies the field type.
var UserMapper = scan.NewMapper[User](
    scan.Column("id",   func(u *User) *int64  { return &u.ID }),
    scan.Column("name", func(u *User) *string { return &u.Name }),
    scan.Column("age",  func(u *User) *int    { return &u.Age }),
)
```

Unknown columns returned by the query are silently discarded -- the mapper only
reads what it knows about. Extra fields in the struct that have no matching
column binding are left at their zero value.

### query: typed execution

All functions accept a `Querier` -- satisfied by both `*query.DB` and `*query.Tx` --
so the exact same call works inside or outside a transaction.

#### QueryAll -- fully materialised

```go
db := query.New(sqlDB)
res := query.QueryAll(ctx, db, UserMapper,
    "SELECT id, name, age FROM users WHERE age > $1", 18)
// result.Result[[]User, error]
users := res.UnwrapOr(nil)
```

#### Query -- lazy cursor-backed Seq[T]

```go
res := query.Query(ctx, db, UserMapper, "SELECT id, name, age FROM users ORDER BY id")
// result.Result[seq.Seq[User], error]
// Rows are closed when the Seq is fully consumed or the consumer breaks early.
names := seq.Map(
    res.Unwrap().Filter(func(u User) bool { return u.Age >= 18 }),
    func(u User) string { return u.Name },
).ToSlice()
```

#### QueryOne -- first row as Option[T]

```go
res := query.QueryOne(ctx, db, UserMapper,
    "SELECT id, name, age FROM users WHERE id = $1", userID)
// result.Result[option.Option[User], error]
// Ok(None) when the row does not exist -- no sql.ErrNoRows to check.
switch {
case res.IsErr():
    return res.UnwrapErr()
case res.Unwrap().IsNone():
    // not found
default:
    u := res.Unwrap().Unwrap()
    fmt.Println(u.Name)
}
```

#### Exec -- INSERT / UPDATE / DELETE / DDL

```go
r := query.Exec(ctx, db, "INSERT INTO users VALUES ($1, $2, $3)", 1, "Alice", 30)
// result.Result[sql.Result, error]
```

### WithTx: railway-oriented transactions

`WithTx` is a single function that handles the entire transaction lifecycle:
commit on `Ok`, rollback on `Err`. The fn's result is returned unchanged.

```go
r := query.WithTx(ctx, db, func(tx *query.Tx) result.Result[int64, error] {
    // tx satisfies Querier -- pass it to any query function directly
    er := query.Exec(ctx, tx,
        `INSERT INTO orders (user_id, total) VALUES ($1, $2)`, userID, total)
    if er.IsErr() {
        return result.Err[int64, error](er.UnwrapErr()) // triggers rollback
    }
    id, _ := er.Unwrap().LastInsertId()
    return result.Ok[int64, error](id) // triggers commit
})
```

### Railway pipeline

Because every function returns a `result.Result`, you can compose them with
`result.Bind` from [go-functionalish](https://github.com/natalie-o-perret/go-functionalish/result):

```go
r := result.Bind(
    query.QueryOne(ctx, db, UserMapper, "SELECT ... WHERE id = $1", id),
    func(opt option.Option[User]) result.Result[Profile, error] {
        u, ok := opt.ValueOk()
        if !ok {
            return result.Err[Profile, error](ErrNotFound)
        }
        return query.QueryOne(ctx, db, ProfileMapper,
            "SELECT ... WHERE user_id = $1", u.ID).
            Map(func(o option.Option[Profile]) Profile { return o.Unwrap() })
    },
)
```

## Design notes

### Why explicit getter functions instead of struct tags?

```go
// struct-tag approach (GORM, sqlx)
type User struct {
    ID   int64  `db:"id"`
    Name string `db:"name"`
}
// go-sql approach
scan.Column("id", func(u *User) *int64 { return &u.ID })
```

Struct tags are strings -- invisible to the compiler, invisible to `go vet`,
invisible to your IDE's rename refactoring. If you rename `User.ID` to `User.UserID`,
the tag silently becomes wrong and you find out at runtime.
With getter functions, the compiler catches the mismatch immediately. The function
body is just `return &u.ID` -- one pointer dereference, zero allocations,
identical to what you'd write by hand.

### Why no code generation?

Code generation (sqlc, Ent, jet) is a valid approach but adds a workflow step
(`go generate` in CI, regenerate after every schema change) and produces files
you didn't write. `go-sql` fits the niche between "full codegen" and "pure
reflection": explicit, compile-time-safe, nothing generated.

### Query vs QueryAll

|                   | `Query[T]`                    | `QueryAll[T]`        |
|-------------------|-------------------------------|----------------------|
| Returns           | `seq.Seq[T]` (lazy)           | `[]T` (eager)        |
| Row scan errors   | Silent (stop iteration)       | Propagated as `Err`  |
| `rows.Err()`      | Not checked                   | Propagated as `Err`  |
| Resource lifetime | Caller controls via iteration | Closed before return |

Use `QueryAll` when you need full error propagation. Use `Query` when you want
to stream large result sets through a `seq` pipeline without materialising everything.

## Dependency graph

```text
scan   =>  (none)
query  =>  scan, go-functionalish/option, go-functionalish/result, go-functionalish/seq
```
