# sqlp

sqlp is a powerputty package to provide extensions to sql.

Primarily driven from experience trying to consolidate too many ways of "doing the right thing" 
when it comes to a persistence layer. 

## Features

* Consistent and minimal "happy path" APIs.
* Contextual transactions to let you write tx agnostic methods cleanly.
* `reflect`ive scanning support using struct tags.
  * Including nested struct and embedded struct support.
* `Repository` pattern support, to provide a wrapper around specific entities.
* Generic struct mapping scanning support to avoid sql tags for performance.
* TODO: Bare minimum, easy to understand query builders (glorified string builders, no extra DSL)

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
func (m *Model) UpdateRow(ctx context.Context, ...) {
  m.Exec(ctx, "UPDATE ...", )
}

m.UpdateRow(ctx, ...) // works directly
m.BeginTx(func(ctx context.Context) error {
  return m.UpdateRow(ctx) // will be ran in transaction!
})
```

### Reflective and Generic Scanning

The Go Wiki shows an [example](https://go.dev/wiki/SQLInterface#getting-a-table) of using reflect to
scan into a struct using `reflect`. However, there are a couple additional features `sqlp` supports:

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
  // Embedded structs are assumed to be columns by default (ie. will write in updates/inserts)
  privateTimestamps // note non-exported embedded struct still has exported fields
  // Timestamps Timestamps `sqlp:,column` -- collision would error
  Timestamps Timestamps `sqlp:timestamps,column`
}

type privateTimestamps Timestamps

type Timestamps struct {
  CreatedAt time.time `sqlp:"created_at"`
  UpdatedAt time.time `sqlp:"updated_at"`
}

// Select into slice
// Will fail before query if Person is not setup correctly
people := []person{}
err := db.Select(ctx, &people, "SELECT * FROM people")

// Get into a struct
p := person{}
err := db.Get(ctx, &p, "SELECT * FROM people LIMIT 1")

// Or for row by row:
// The first scan caches the reflection for performance, so must be called with same destination
rows, err := db.Query(ctx, "SELECT * FROM people")
scanner := NewReflectScanner[person](rows)
for rows.Next() {
  p, err := scanner.Scan()
}
```

### Repository pattern

`sqlp` provides a repository pattern to provide nicer APIs on top of `sqlp.DB`. By using generics
and declaring your target struct as a type parameter, you can:

* verify the struct tags are setup correctly ad hoc (such as during initialization)
* get and select values directly instead of passing in pointers
* better performance since we can fill result slices without reflection (though reflection 
  is still used for scanning).

```go
repository := sqlp.NewRepository[person](db, "people")
if err := repository.Validate(); err != nil {
  log.Panicf("people struct is not setup correctly: %v", err)
}
people, err := repository.Select(ctx, "SELECT * FROM people")
person, err := repository.Get(ctx, "SELECT * FROM people LIMIT 1")
person, err := repository.Find(ctx, 1) // SELECT * FROM people WHERE id = 1 LIMIT 1
```

### Generics only support

Reflect is very useful for helping make declarative models and clean your code, but ultimately may 
be too slow for your purposes. Generics allow us to approach similar goals without the performance
overhead. Mappers handle mapping the column names, to the target address in our object that we
want to scan into.

Note, currently mappers don't nil out zero embeds, so for example, if you are selecting columns
into an embed like pet, you should be aware of those zero structs.

```go
petMapper := Mapper[pet]{
  "id":   func(p *pet) any { return &p.ID },
  "name": func(p *pet) any { return &p.Name },
  "type": func(p *pet) any { return &p.Type },
}
personMapper := Mapper[person]{
  "id":         func(p *person) any { return &p.ID },
  "first_name": func(p *person) any { return &p.FirstName },
  "last_name":  func(p *person) any { return &p.LastName },
}
// Support pet
personMapper = MergeMappers(personMapper, petMapper, "pet", func(p *person) *pet {
  if p.Pet == nil {
    p.Pet = &pet{}
  }
  return p.Pet
})
// Support children
personMapper = MergeMappers(personMapper, personMapper, "child", func(p *person) *person {
  if p.Child == nil {
    p.Child = &person{}
  }
  return p.Child
})
```

## Brainstorm

Brainstorming concerns that influence the design of this module and suggestions.

### Scanning

Scanning is a big subject, and sqlp tries to support multiple strategies. However, we must 
acknowledge some limitations with having a consistent API across these strategies. Because go
doesn't support method generics, we need a layer outside the `DB` connection for those APIs. 
Conversely, scanning into a destination does not -- `sqlp` unconventionally provides distinct API
options, and it's up to developer which strategies to adopt -- ideally you just choose one option 
and stick to it for consistency.

### Row

Because `sql.Row` doesn't provide `sql.Rows`, and therefore no way to get column names, we can't
really provide any of the niceties without re-implementing it entirely. For now, this package avoids
doing that, and you can just use the other APIs.

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