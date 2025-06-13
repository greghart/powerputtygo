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
* `errcmp` -- error matcher for tests ([source](https://github.com/google/exposure-notifications-server/blob/main/pkg/errcmp/errcmp.go))
