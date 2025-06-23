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

### Single Path -- Exec, Query, QueryRow

Forgo having the separate contextless method, and keep your team from accidentally writing 
un-cancelable long running queries, with a simple and reduced surface area:

```go
db, err := sqlp.Open("sqlite", ":memory:") // same API as sql.Open, just returns a sqlp.DB
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
* write support -- update and insert structs
  * we want to avoid becoming an ORM, so this is an intentionally thin and basic layer, just
    helping write concise code for the basic cases
  * by default, all non-struct type fields are assumed to be direct columns, as well as fields
    of anonymous embedded structs
  * set `promote` tag to promote a struct's fields upwards
  * set `readonly` tag to signal a field should only be read (aggregates, auto gens, etc.)

```go
type Person struct {
  // Column default from field name 'ID'
  ID int
  // Column specified with struct tag
  Name string `sqlp:"name"`
  // Column that won't be written in writes, but will be read (eg. `COUNT(*) AS num_children`)
  NumChildren int `sqlp:"num_children,readonly"`
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

// Note because golang has no generic function methods (https://github.com/golang/go/issues/49085),
// we provide separate generic functions that take db as a parameter

// Select into slice
// Will fail before query if Person is not setup correctly
people, err := sqlp.Select[person](ctx, db, "SELECT * FROM people")

// Get into a struct
person, err := sqlp.Get[person](ctx, db, "SELECT * FROM people LIMIT 1")

// Or for row by row:
// The first scan automatically caches all reflection for performance
rows, err := sqlp.Query[person](ctx, "SELECT * FROM people")
for rows.Next() {
  p, err := rows.Scan()
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

rows.SetMapper(personMapper)
for rows.Next() {
  p, err := rows.Scan() // p is a person!
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
based on user requirements. Scanning as a term is used in the same way as `database/sql` -- copying 
one row of data into some values. While `database/sql` asks you to scan into pointers, `sqlp` 
asks you to define what your destination is using generics. We then use that to automatically
reflect and populate your desired targe.

Because go doesn't support method generics, we need a layer outside the `DB` connection for 
generic APIs. If you don't want this, you are of course welcome to ignore these APIs completely.

#### Column Mapping 

How do we map column names to our structs? For column `foo`, which field of which struct should we
target?

`sqlp` provides support for standard mapping using struct tags, as well as defining custom generics
mappers to manually thread data into arbitrarily embedded struct fields.

Ultimately, this boils down to a decision of which "scanner" you'd like to use -- `ReflectScanner`
(default), or defining a manual column `Mapper`. Both `sqlp.Rows` and `sqlp.Repository` let you 
opt-in to a mapper, which will be used over reflection for the performance benefit.

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