module github.com/greghart/powerputtygo/sqlp

go 1.24.1

require (
	github.com/google/go-cmp v0.7.0
	github.com/greghart/powerputtygo/errcmp v0.0.0-00010101000000-000000000000
	github.com/lib/pq v1.10.9
	github.com/mattn/go-sqlite3 v1.14.28
)

replace github.com/greghart/powerputtygo/errcmp => ../errcmp
