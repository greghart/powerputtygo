# sqlp

sqlp is a powerputty package to provide extensions to sql.
Inspired by [sqlx](https://jmoiron.github.io/sqlx/)

## Features

* Consistent and single correct path APIs
* Contextual transactions to let you write tx agnostic methods
* Reflective scanning support with `DB.Get` and `DB.Select`
  * Including embed support
* Generics support with `Model`

## Examples

### Exec, Query, QueryRow

Forgo having the separate contextless method, and instead use these directly with context.

```go
db.Exec(ctx, query, ...args)
```

### Contextual Transactions

All methods on DB support contextual transactions, letting you write methods that are totally 
agnostic to whether they're being ran in a transaction, and also don't need the transaction passed 
into them as an argument

```go
type Model struct {
  *sqlp.DB
}
...
func (m *Model) UpdateRow(ctx, ...) {
  m.Exec(ctx, "UPDATE ...", )
}

m.UpdateRow(ctx, ...) // works directly
m.BeginTx(func(ctx context.Context) {
  m.UpdateRow(ctx) // will be ran in transaction!
})
```

### Reflect

The Go Wiki shows an [example](https://go.dev/wiki/SQLInterface#getting-a-table) of using reflect to
scan into a struct using `reflect`. However, there are a couple additional features we want to 
support:

* field / column mapping
  * we want to allow custom mapping of columns to fields, in cases where order doesn't match
* partial select 
  * we want to select a subset of the struct to fill in
* embedded fields
  * we want to have embedded structs populated by the query results
* write support -- annotate which fields should "belong" to the table of the struct
  * by default, only top level fields which aren't sub structs are written
  * add `write` to override to writing, or `readonly` 

```go
type Person struct {
  // Column default from field name 'ID'
  ID int
  // Column specified with struct tag
  Name string `sqlp:"name"`
  // Embedded structs should come in `_` separated (configurable)
  // Eg. child1_name, child2_name
  Child1 *Person `sqlp:"child1"`
  Child2 *Person `sqlp:"child2"`
  Ignore *Person `sqlp:"-"` // will never be scanned
  unexported *Person // Not reflectable
  // Anonymous structs not prefixed or separated at all
  // Collisions will error
  Timestamps `sqlp:,write`
  privateTimestamps // Does still work since non-exported embedded struct has exported fields
}

type privateTimestamps Timestamps

type Timestamps struct {
  CreatedAt time.time `sqlp:"created_at"`
  UpdatedAt time.time `sqlp:"updated_at"`
}
```
