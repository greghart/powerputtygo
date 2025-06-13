# queryp

`queryp` is a powerputty package to provide minimal, composeable, non-magical helpers for writing
complex SQL queries in a more maintainable and readable way.

## Goals and Features

* Named parameter query builders (regardless of driver support)
* Dynamic and re-usable queries using extensions to `text/template` package

### Named Parameters

To help build larger queries with lots of placeholders, use named parameters even if your db driver
doesn't support them. This just does dead simple string replacement of `:label` placeholders with
database driver placeholders, still utilizing placeholder args for proper escaping and validation.

defaults to `?` placeholder (eg. sqlite):
```go
q := queryp.Named("SELECT * FROM test WHERE id = :id AND name = :name").
  Map(map[string]any{
    "name": "Alice",
    "id":   1,
  })
// q.String() == "SELECT * FROM test WHERE id = ? AND name = ?"
// q.Args() == []any{1, "Alice"}
rows, err := c.Query(ctx, q.String(), q.Args()...)
```

postgres is also supported:
```go
q := queryp.Named("SELECT * FROM test WHERE id = :id AND name = :name").
  Map(map[string]any{
    "name": "Alice",
    "id":   1,
  }).
  WithPlaceholderer(queryp.PostgresPlaceholderer)
// q.String() == "SELECT * FROM test WHERE id = $1 AND name = $2"
// q.Args() == []any{1, "Alice"}
rows, err := c.Query(ctx, q.String(), q.Args()...)
```

### Query Helpers

Apart from named parameters, building complex queries with dynamic portions can also be trying.
We provide a wrapper around the standard library `text/template` to help with this:

* `Execute` out strings and args instead of writing to a buffer
* Built-in support for named parameters through above syntax
* Other helpers that work with the builder syntax for common use cases
  * `.Params` -- function that returns all defined params (if any)
  * `.Param "param"` -- function that returns placeholder for param
  * `.Includes "association"` -- function that returns whether any of the given "association" is 
    included (lets you include some shared join if either association is included for example)

```go
getPeopleAndPets := queryp.Must(queryp.NewTemplate(
  "getPeopleAndPets", 
  `
    SELECT
      p.id, p.first_name, p.last_name
      {{if .Includes "pet" -}},
      COALESCE(pet.id, 0) AS pet_id,
      COALESCE(pet.name, "") AS pet_name
      {{- end}}
    FROM people p
    {{if .Includes "pet" -}}
    LEFT JOIN pets pet ON pet.parent_id = p.id
    {{- end}}
    WHERE 
      p.id = {{.Param "id"}}
      OR p.id = :id /* Both work! */
  `))

q, args := getPeopleAndPets.Includes("pet").Param("id", 1).Execute() 
q, args := getPeopleAndPets.Param("id", 1).Execute()

```

## More Context

Brainstorming and additional context that influenced the design of this module.

### Query Helpers Discussion

The go ecosystem is rife with APIs for building SQL queries in a more maintainable way. 

Two of the most popular are:

* `squirrel` -- builder style API
* `sqlc` -- compile (mostly) straight SQL

Both of these have trade offs: `squirrel` is really good at decomposing your queries, excellent for
highly dynamic use cases where you've got a bunch of parameters to your query building. However,
your queries can end up sprawled out, making it hard to grok what your SQL actually is. Conversely,
`sqlc` is excellent because you don't have to learn a new DSL, and it lets you flex your SQL skills
and build really complicated queries that just work. However, support for dynamic parameters is 
[indirect](https://github.com/sqlc-dev/sqlc/discussions/364) and comes with real costs.

This package tries to thread the needle in between these options, using a layer on top of go's 
standard `template/text` package to let you write basic SQL like you're used to, but with a bit
of frosting to make common cases a bit easier to handle. Finally, we of course will attempt to
compose nicely with other `powerputty` packages.