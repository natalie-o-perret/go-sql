package query_test

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/natalie-o-perret/go-functionalish/result"
	gosqlquery "github.com/natalie-o-perret/go-sql/query"
	"github.com/natalie-o-perret/go-sql/scan"
	_ "modernc.org/sqlite"
)

type User struct {
	ID   int64
	Name string
	Age  int
}

var userMapper = scan.NewMapper[User](
	scan.Column("id", func(u *User) *int64 { return &u.ID }),
	scan.Column("name", func(u *User) *string { return &u.Name }),
	scan.Column("age", func(u *User) *int { return &u.Age }),
)

func openTestDB() *gosqlquery.DB {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	return gosqlquery.New(sqlDB)
}
func ExampleQueryAll() {
	db := openTestDB()
	ctx := context.Background()
	gosqlquery.Exec(ctx, db, `CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, age INTEGER)`)
	gosqlquery.Exec(ctx, db, `INSERT INTO users VALUES (1, "Alice", 30), (2, "Bob", 17)`)
	res := gosqlquery.QueryAll(ctx, db, userMapper, "SELECT id, name, age FROM users WHERE age >= 18")
	for _, u := range res.UnwrapOr(nil) {
		fmt.Printf("%s (%d)\n", u.Name, u.Age)
	}
	// Output:
	// Alice (30)
}
func ExampleQueryOne_found() {
	db := openTestDB()
	ctx := context.Background()
	gosqlquery.Exec(ctx, db, `CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, age INTEGER)`)
	gosqlquery.Exec(ctx, db, `INSERT INTO users VALUES (1, "Alice", 30)`)
	res := gosqlquery.QueryOne(ctx, db, userMapper, "SELECT id, name, age FROM users WHERE id = ?", 1)
	if res.IsOk() && res.Unwrap().IsSome() {
		fmt.Println(res.Unwrap().Unwrap().Name)
	}
	// Output:
	// Alice
}
func ExampleWithTx() {
	db := openTestDB()
	ctx := context.Background()
	gosqlquery.Exec(ctx, db, `CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, age INTEGER)`)
	r := gosqlquery.WithTx(ctx, db, func(tx *gosqlquery.Tx) result.Result[int64, error] {
		er := gosqlquery.Exec(ctx, tx, `INSERT INTO users VALUES (1, "Alice", 30)`)
		if er.IsErr() {
			return result.Err[int64, error](er.UnwrapErr())
		}
		id, _ := er.Unwrap().LastInsertId()
		return result.Ok[int64, error](id)
	})
	fmt.Println(r.Unwrap())
	// Output:
	// 1
}
