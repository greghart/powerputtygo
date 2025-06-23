# clientp

`clientp` is a powerputty package to provide robust and flexible clients -- wrappers around 
any other service or package, where utilities like retries, automatic logging, error handling, 
batching, etc. could be useful.

## Goals and Features

* Generic and re-usable batch client that can transaparently batch operations together.
* Tiered client that can waterfall multiple implementations together transparently. TODO
* Caching client with support for any `cachep` cache. TODO

### Batch Client

A generic client that will batch up requests by some id, and throttle resolutions of them in bulk 
every `x` duration (or with a limit of up to `k` batch size).

```go
// say you have a database that can do
user, err := usersDB.Find(1)
users, err := usersDB.WhereID([]int{1})

// ID must be comparable, no constraint on Result
batchedUsers := clientp.NewBatch(
  func(ids []int64) (map[int64]User, error) {
    // Whatever glue you need here to get results by ID
    // This lets us have a `Result` that's not necessarily implementing some arbitrary interface
    results, err := usersDB.WhereID(ids)
    if err != nil {
      return nil, err
    }

    out := make(map[int64]User, len(results))
    for _, res := range results {
      out[res.ID] = res
    }
    return out, nil
  },
  BatchOptions{Timeout: 50 * time.Millisecond},
)
batchedUsers.Start() // Ensure Start to process batches in goroutine
defer batchedUsers.Stop()

idsToGet := []int64{1,2,3}
for _, id := range idsToGet {
  // These will batch together to a `WhereID`
  go func() {
    user, err := batchedUsers.Execute(id) 
  }()
}

////////////////////////////////////////////////////////////////////////////////
// suggested pattern: embed underlying client and override methods, to make interface transparent
type batchedUsers struct {
  *UsersDB
  *clientp.Batch
}

func newBatchedUsers(users *usersDB) *batchedUsers {
  return &batchedUsers{
    users,
    clientp.NewBatch(...),
  }
}

func (bu *batchedUsers) Find(id int64) (*User, error) {
  return bu.UsersDB.Find(id)
}

user, err := bu.Find(1)
```