# servicep

`servicep` is a powerputty package to provide minimal, composeable helpers for writing web services
in a more maintainable and readable way. We are not aiming to replace web frameworks like gin, but
instead providing even lower level components that can be used agnostically across frameworks.

## Goals and Features

* standardized utilities for composing RESTful or RPC style APIs
  * sparse fieldsets TODO
  * relationship inclusion

### Inclusions

In RESTful web services, as well as entity-oriented RPC services, you often need to support 
including relationships of an entity in a response, to avoid multiple round trips. 
This must be passed through the API to the services and persistence level (eg. to know to map into 
multiple requests or a join, etc.).

`servicep` offers `Includable` as a simple way to define and query inclusions.

```go
i := NewIncludable()
```