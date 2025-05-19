# sqlp

sqlp is a powerputty package to provide extensions to sql.

Primarily driven from experience trying to consolidate too many ways of "doing the right thing" 
when it comes to a persistence layer. 

## Features

* Consistent and minimal "happy path" APIs
* Contextual transactions to let you write tx agnostic methods cleanly.
* Reflective scanning support for more flexible queries without the ORM
  * Including nested struct and embedded struct support
* Taken a step further, generics support with `Model`, for ORM-lite behavior without subscribing
  to a large framework.

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
* recursive struct fields
  * we want to have struct fields themselves populated by the query results
* write support -- update and insert structs
  * we want to avoid becoming an ORM, so this is an intentionally thin and basic layer, just
    helping write concise code for the basic cases
  * by default, all non-struct type fields are assumed to be direct columns, as well as fields
    of embedded structs
  * set `column` tag to set a struct's fields as columns 
  * set `virtual` tag to remove a non-struct type field as a column

```go
type Person struct {
  // Column default from field name 'ID'
  ID int
  // Column specified with struct tag
  Name string `sqlp:"name"`
  // Column that won't be written in writes, but will be read
  NumChildren int `sqlp:"num_children,virtual"`
  // structs should come in `_` separated (configurable)
  // Eg. child1_name, child2_name
  Child1 *Person `sqlp:"child1"`
  Child2 *Person `sqlp:"child2"`
  Ignore *Person `sqlp:"-"` // will never be scanned
  unexported *Person // Not reflectable
  // Collisions will error
  Timestamps Timestamps `sqlp:timestamps,column`
  // Embedded structs are assumed to be columns by default (ie. will write in updates/inserts)
  privateTimestamps // note non-exported embedded struct still has exported fields
}

type privateTimestamps Timestamps

type Timestamps struct {
  CreatedAt time.time `sqlp:"created_at"`
  UpdatedAt time.time `sqlp:"updated_at"`
}
```

## Brainstorm

Brainstorming concerns that influence the design of this module and suggestions.

### Sub structs

Having large structs that with sub-structs as relationships is a common use case. 
Eg. our case above, a parent can have a child, but doesn't always, so a pointer to a child is 
natural (yes it should probably be a slice of children!)

The difficulty lies in scanning when selecting a parent and left joining their children in. 
* If there is a child, we need to set one up to have values to scan into
* If there is not a child, we will have nulls left joined, that have to scan *somewhere*

This packages suggests handling this by utilizing COALESCE in your queries, to let scanning have
one path. `sqlp` will setup any embed that is being selected into for a query -- it will then
clean up any of these that were only populated with zero values.

### Field/column/parameter order 

One of the maintainability concerns with using vanilla `sql` is the requirements to keep the order
of your fields (whether selecting or using args) coordinated. For basic examples this doesn't tend
to be a problem, but for more advanced queries with tens of fields or arguments, refactoring
becomes error prone and manual.

TODO: Introduce the params placeholder struct.

### Keep the ingredients simple

Taking inspiration from sqlc, we're not trying to write a non SQL DSL for making queries. Even
sqlc introduces its' own DSL for macros, like conditional filtering.

We're also not introducing any code gen.

Ideally, you can just write your queries using basic go, and use a couple utility structs to help
make it maintainable.

TODO: Show our version of query building