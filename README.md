# powerputty

powerputty Go is a collection of general use packages, to help build web
services with Golang.

powerputty is built as a multi-module [workspace](https://go.dev/doc/tutorial/workspaces).
All packages are appended with `p` to help avoid naming collisions (eg. don't collide with 
stdlib `sql` package, use `sqlp` or "SQL Putty")

I'm also just using it as a central place to keep my learnings and knowledge in a structured place.

## Packages

* Persistence layer
  * A primary goal here is to avoid becoming an ORM. There should be relatively little magic;
    we are building composable tools, not prescribing an ORM solution
  * [sqlp](./sqlp/README.md) SQL extensions
  * [queryp](./queryp/README.md) Helpers to write SQL queries more cleanly
  * [mapperp](./mapperp/README.md) Map flat rows of data into domain models, "orm lite"
* [servicep](./servicep/README.md) Web service utilities to help your REST / gRPC / other APIs.
* [clientp](./clientp/README.md) Convenient helpers for consuming (ie. being a client)
* [utilp](./utilp/README.md) More general utilities, eg. basic generic map function
* `errcmp` error matcher for tests 
  * ([source](https://github.com/google/exposure-notifications-server/blob/main/pkg/errcmp/errcmp.go))
  * slight addition for `extra` context

## Observability

### Logging

When applicable, all packages take in a `slog.Logger` instance to customize logging. By default,
only logs to `slog.LevelDebug`.

### Metrics

When applicable, powerputty packages will take  `Metric` interfaces that align with prometheus
metric types.

* `MetricCounter` -- corresponds to `prometheus.NewCounter`
* `MetricHistogram` -- corresponds to `prometheus.NewHistogram`