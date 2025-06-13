# mapperp

`mapperp` is a powerputty package to help map tabular (row-based) results into nested relational
domain models (ie. the "Data Mapper" pattern).

Note that this process is similar to a map-reduce. For every row, we process, transform, and
potentially reduce into a new output. This also isn't directly related to sql, but we'll see that
it's quite useful for our `sqlp` package.

_Quick semantic note; don't confuse with `sqlp.Mapper`, which is used for mapping column names to
addresses for scanning purposes._

## Goals and Features

* Help cutdown on scaffolding when querying large amounts of data across tables from sql
* Declaratively show what's happening in a non magical way

## Background

As a contrived case, let's say you are wanting to join a person to their pets. The database spits
back rows with both person data and pet data. If a person has more than one pet, their data will
span more than one row, eg:

```
| person_id | person_name | pet_id | pet_name |
| 1         | Albert      | 1      | Kitty    |
| 1         | Albert      | 2      | Doggy    |
```

See [here](../sqlp/example_one_to_many_test.go) for an example of how to do this manually.
There's nothing wrong with this, and in the go style, it's clear, performant, and has obvious flow.
Many ORMs on the other hand will wrap this into "eager load" or "preload" APIs, where it's not 
apparent what SQL is being ran or how the rows are being mapped into your domain models. 

If you have many such cases though, or queries with deeply nested joined data, it may feel hard to 
maintain. That's where `mapperp` comes in.

## Example

`mapperp` boils down to a few basic ingredients:
* A mapper function is ran on every row, and internally transforms and aggregates data.
* The mapper aggregates into an `out` (or memo) value, which is available at each scan.
* Identifier functions dictate how to uniquely identify entities across rows.
* Getter functions for mapping a single row to your specific entity.
* Various functions to help you compose your mapper: expected row counts, associations, etc.

```go
	cragMapper := mapRow(
		singleMapper(
			func(row *cragRow) *models.Crag { return &row.Crag },
		),
		manyMapper(
			func(e *models.Crag) *[]models.Area { return &e.Areas },
			func(row *cragRow) *models.Area { return &row.Area },
			func(e *models.Area) int64 { return e.ID },
		),
		manyMapper(
			func(e *models.Crag) *[]models.Boulder {
				if len(e.Areas) == 0 {
					return nil // no areas, no boulders
				}
				return &e.Areas[len(e.Areas)-1].Boulders // get boulders from the latest area
			},
			func(row *cragRow) *models.Boulder { return &row.Boulder },
			func(e *models.Boulder) int64 { return e.ID },
		),
	)
```

### Identifier

0 value of identifier is assumed to be a "null" value in our data, and should not result in a new
entity. If this behavior doesn't work for your use case, please open an issue.