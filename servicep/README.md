# servicep

`servicep` is a powerputty package to provide minimal, composeable helpers for writing web services,
whether RESTful or RPC, in a more maintainable and readable way. We are not aiming to replace web 
frameworks like gin, but instead providing low level components that can be used agnostically across
frameworks.

## Goals and Features

* standardized utilities for composing RESTful or RPC style APIs
* relationship/association inclusions
* field masks support for sparse reads and "PATCH"ful updates
* errgroup helpers for common patterns

### Inclusions

In RESTful web services, as well as entity-oriented RPC services, you often need to support 
including relationships of an entity in a response, to avoid multiple round trips. 
This must be passed through the API to the services and persistence level (eg. to know to map into 
multiple requests or a join, etc.).

`servicep` offers `IncludeRequest` and `IncludeSchema` as a simple way to define allowed inclusions, 
and work with user level include requests.

```go
// Static include schema
CountryIncludeSchema := servicep.NewIncludeSchema().Allow(
  "states.counties.cities",
  "states.flags",
  "regions",
)
CountryIncludeSchema.IsAllowed("states") // true
CountryIncludeSchema.IsAllowed("states.counties.cities") // true
CountryIncludeSchema.IsAllowed("cities") // false

// User level requests
userRequests = []string{"states.counties", "regions", "dogs"}
req = CountryIncludeSchema.Include(userRequests...)
req.IsAllowed("states.counties") // true
req.IsAllowed("states.counties.cities") // false
req.IsAllowed("dogs") // false, not in schema
```

### Field Masks

[Field masks](https://google.aip.dev/161) are a common way to support PATCH requests in RESTful 
services, similar mutations in RPC, and improving performance in read APIs to filter down the result
set. JSON APIs often support similar behavior implicitly:
* undefined -> don't update at all
* null -> clear the field
* value -> update the field

However, this behavior is not always clear, and easy to mess up for clients. Additionally, working 
in non Javascript languages that don't have the null/undefined distinction can make integrations
harder to understand and more brittle. 

Field masks make this behavior explicit. A user can specify fields, nested fields (with a similar
syntax to inclusions), or wildcards (TODO). Working against this list makes logic clear and trivial:

```go
type UpdateRequest struct {
  *servicep.FieldMask
  Name string
  Author Author
  Publisher Publisher
}
type Author struct {
  Name string
  Secret string
}
type Publisher struct {
  Name string
}
// if using gRPC:
mask := NewFieldMaskFromPB(protobuf.Mask /* *fieldmaskpb.FieldMask */)

// basic REST or slice:
// expect on name, author name, or anything about publisher ('publisher.*' == 'publisher')
mask := NewFieldMask([]string{"name", "author.name", "publisher.*"})
mask.Has("name") // true
mask.Has("publisher.name") // true
mask.Has("author.secret") // false
```

Note, we are not providing any reflection to make these field names automagically integrate with 
struct fields.

### Errgroup helpers

[errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup) "provides synchronization, error 
propagation, and Context cancelation for groups of goroutines working on subtasks of a common task.",
and is a very useful construct for services that nee to orchestrate multiple tasks for an endpoint.

`servicep` just adds some additional generics helpers to compose them up more easily.

#### ErrGroupValuer

`ErrGroupValuer` adds support for running functions that return both an error and value. The value
will be passed to the channel returned by this function.

```go
fetchURL := func(url string) func() (http.)
var g errgroup.ErrGroup
ErrGroupValuer(g, 
```