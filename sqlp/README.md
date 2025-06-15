# sqlp

`sqlp` is a powerputty package to provide extensions to the `sql` package.

Primarily driven from experience trying to consolidate too many ways of "doing the right thing" 
when it comes to the persistence layer. Note we are only concerned with behaviors interacting with
the database directly -- query building helpers can be found in [queryp](../queryp/README.md)

## Goals and Features

* Consistent and minimal "single path" APIs.
* Contextual transactions to let you write tx agnostic methods cleanly.
* `reflect`ive scanning support using struct tags.
  * Including nested struct and embedded struct support.
* `Repository` pattern support, to provide a wrapper around specific entities.
* Generic struct mapping scanning support to avoid sql tags for performance.

## Examples

### Single Path -- Exec, Query, QueryRow

Forgo having the separate contextless method, and keep your team from accidentally writing 
un-cancelable long running queries, with a simple and reduced surface area:

```go
db, err := sqlp.Open("sqlite", "./test.db") // same API as sql.Open, just returns a sqlp.DB
if err != nil {
  log.Panicf("failed to connect to database: %v", err)
}

db.Exec(ctx, query, ...args)
db.Query(ctx, query, ...args)
db.QueryRow(ctx, query, ...args)
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
  m.Exec(ctx, "UPDATE ...", ...)
}

m.UpdateRow(ctx, ...) // works directly
m.RunInTx(func(ctx context.Context) error {
  return m.UpdateRow(ctx, ...) // will be ran in transaction!
})
```

### Reflective Scanning

The Go Wiki shows an [example](https://go.dev/wiki/SQLInterface#getting-a-table) of using reflect to
scan into a struct using `reflect`. `sqlp` expands on this idea with a couple additional features:

* field / column mapping
  * we want to allow custom mapping of columns to fields, in cases where order doesn't match or 
    isn't easily predictable
* partial select 
  * we want to select a subset of the struct to fill in
* embedded struct fields
  * we want to have embedded structs also populated by the query results
* TODO: write support -- update and insert structs
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
  // Column that won't be written in writes, but will be read (eg. `COUNT(*) AS num_children`)
  NumChildren int `sqlp:"num_children,virtual"`
  // structs should come in `_` separated (configurable)
  // Eg. child1_name, child2_name
  Child1 *Person `sqlp:"child1"`
  Child2 *Person `sqlp:"child2"`
  Ignore *Person `sqlp:"-"` // will never be scanned
  unexported *Person // Not reflectable
  // Embedded structs are promoted by default (ie. will be read as `created_at`/`updated_at` and
  // written as well)
  privateTimestamps // note non-exported embedded struct still has exported fields
  // Timestamps Timestamps `sqlp:,promote` -- collision would error, so we can namespace these
  Timestamps Timestamps `sqlp:"timestamps,promote"`
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

### Generics/mapping scanning support

Reflect is very useful for helping make declarative models, but ultimately may be too slow or 
fragile for your purposes. Generics allow us to approach similar goals without the performance or
abstraction overhead. We can use mappers to handle mapping column names to target addresses in our 
struct that we want to scan into.

```go
petMapper := sqlp.Mapper[pet]{
  "id":   func(p *pet) any { return &p.ID },
  "name": func(p *pet) any { return &p.Name },
  "type": func(p *pet) any { return &p.Type },
}
personMapper := sqlp.Mapper[person]{
  "id":         func(p *person) any { return &p.ID },
  "first_name": func(p *person) any { return &p.FirstName },
  "last_name":  func(p *person) any { return &p.LastName },
}
// Support pet
personMapper = sqlp.MergeMappers(personMapper, petMapper, "pet", func(p *person) *pet {
  if p.Pet == nil {
    p.Pet = &pet{}
  }
  return p.Pet
})
// Support children
personMapper = sqlp.MergeMappers(personMapper, personMapper, "child", func(p *person) *person {
  if p.Child == nil {
    p.Child = &person{}
  }
  return p.Child
})

scanner := sqlp.NewMappingScanner(rows, personMapper)
for rows.Next() {
  p, err := scanner.Scan() // p is a person!
  if err != nil {
    log.Panicf("failed to scan row: %v", err)
  }
}
```

Note for these APIs, we must manually "touch" (initialize a 0 value of) any embedded struct that 
we're scanning into.
Similarly, it would be up to consumer to nil out any such structs that are zero values after the 
fact.

## More Context

Brainstorming and additional context that influenced the design of this module.

### Scanning

Scanning is a big subject, and sqlp tries to support multiple strategies. Because of this 
complexity, it's important to have clear semantics, so that it's obvious which APIs to use and why
based on user requirements. Scanning is used in the same way as `database/sql` -- copying one row 
of data to some value pointers.

There are two basic concepts for scanning sql rows: destination setup and column mapping.

#### Destination

Where does our data end up?

`sqlp` provides support to setup structs automatically as a destination for your sql data.
There are two potential paradigms to achieve this: using generics, or using reflection.
Because go doesn't support method generics, we need a layer outside the `DB` connection for 
generic APIs. Conversely, reflective scanning into a destination does not need generics, and can
be attached as methods directly to the `DB`.

#### Column Mapping 

How do we map column names to our structs? For column `foo`, which field of which struct should we
target?

`sqlp` provides support for mapping columns into arbitrarily embedded struct fields.
Similarly to destination, we have reflective and generic APIs for this.

* Reflective column mapping -- `DB.Get` / `DB.Select` / `Get` / `Select` / `ReflectScanner`/ `Repository`
* Generic column mapping -- `Mapper` / `MappingScanner`

#### Table

| Method | Destination | Column | Notes |
| ------ | ----------- | ------ | ----- |
| `DB.Get` | Reflect | Reflect | Scan into a destination struct |
| `DB.Select` | Reflect | Reflect | Scan into a destination slice of structs |
| `ReflectDestScanner` | Reflect | Reflect | Used under the hood by `DB.Get` and `DB.Select` |
| `Get` | Generic | Reflect | Function to scan out a destination struct |
| `Select` | Generic | Reflect | Function to scan out a destination slice of structs |
| `Repository` | Generic | Reflect | |
| `ReflectScanner` | Generic | Reflect | Used by `Repository` |
| `MappingScanner` | Generic | Generic | Only row by row scanning supported for now |

### Row

Because `sql.Row` doesn't provide `sql.Rows`, and therefore no way to get column names, we can't
really provide any of the niceties without re-implementing it entirely. For now, this package avoids
doing that, and you can just use the other APIs.

### Relationships (OneToOne / Embedded Structs)

Using embedded structs to model hasOne relationships is a common use case in many domains.
Eg. in our case above, a parent can have a child, but doesn't always, so a pointer to a child is 
natural.

The difficulty lies in scanning when selecting a parent and left joining their children in. 
* If there is a child, we need to set one up to have values to scan into
* If there is not a child, we will have nulls left joined, that have to scan *somewhere*

Because of this, `sqlp` reflect methods will automatically touch nil embedded pointer structs if
it detects we're scanning into those fields. For the generic case, you can handle this manually.
This packages suggests handling this by utilizing COALESCE in your queries, to let scanning have
one path. `sqlp` will setup any embed that is being selected into for a query -- it will then
clean up any of these that were only populated with zero values.

### Relationships (OneToMany / Embedded Slices)

Similar to above, you may have a slice of children instead of a single child. The expectation could
be to join a children table, and populate that slice with whatever children come in the result set.

The complexity of handling this is much higher than the OneToOne case -- here, you would have 
multiple rows that all correspond to one result.

Because our goal is not to become an ORM, we don't handle this case manually.  but do showcase ways
to handle this in the one to many example (see code/godoc).

Additionally, powerputty provides the `mapperp` package to help you map data across multiple rows
into your domain entities, with support for one to many use cases.

### Field/column/parameter order 

One of the maintainability concerns with using vanilla `sql` is the requirements to keep the order
of your fields (whether selecting or using args) coordinated. For basic examples this doesn't tend
to be a problem, but for more advanced queries with tens of fields or arguments, refactoring
becomes error prone and manual.

powerputty provides the `queryp` package to help coordinate query readability and maintainablity.

### Keep the ingredients simple

Taking inspiration from sqlc, we're not trying to write a new SQL DSL for making queries. Even
sqlc introduces its' own DSL for macros, like conditional filtering.

We're also not introducing any code gen.

Ideally, you can just write your queries using basic go, and use a couple utility structs to help
make it maintainable.