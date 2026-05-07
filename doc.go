// Package gosql is a zero-reflection, generics-native SQL library for Go 1.24+.
//
// No reflection. No struct tags. No code generation. Pure generics, explicit
// column bindings, and railway-oriented error handling via Result[T,E].
//
// It provides two packages:
//
//   - [github.com/natalie-o-perret/go-sql/scan]: Mapper[T] — explicit column → field bindings
//   - [github.com/natalie-o-perret/go-sql/query]: Query[T], QueryOne[T], QueryAll[T], Exec, WithTx[T]
//
// Designed as a companion to [github.com/natalie-o-perret/go-functionalish]:
// query results are returned as result.Result[seq.Seq[T], error] or
// result.Result[option.Option[T], error], enabling seamless railway pipelines.
package gosql
