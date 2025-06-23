package clientp_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/greghart/powerputtygo/clientp"
	"github.com/greghart/powerputtygo/sqlp"
	_ "github.com/mattn/go-sqlite3"
)

// assuming people database is already setup
type user struct {
	ID   int64
	Name string
}
type usersDB struct {
	*sqlp.Repository[user]
	*clientp.Batch[int64, user]
}

func NewUsersDB(db *sqlp.DB) *usersDB {
	repo := sqlp.NewRepository[user](db, "users")
	u := &usersDB{
		Repository: repo,
	}
	u.Batch = clientp.NewBatch[int64, user](u.resolveBatch)
	return u
}

func (u *usersDB) resolveBatch(ctx context.Context, ids []int64) (map[int64]user, error) {
	users, err := u.Select(ctx, "SELECT * FROM users WHERE id IN (?)", ids)
	slices.Sort(ids)
	fmt.Printf("batch: %v\n", ids)
	if err != nil {
		return nil, err
	}

	out := make(map[int64]user, len(users))
	for _, res := range users {
		out[res.ID] = res
	}
	return out, nil
}

func (u *usersDB) Find(ctx context.Context, id int64) (*user, error) {
	return u.Execute(ctx, id)
}

func Example_batch() {
	db, err := sqlp.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}

	usersDB := NewUsersDB(db)
	usersDB.Start()
	defer usersDB.Stop()

	idsToGet := []int64{1, 2, 3}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	done := make(chan any)
	go func() {
		var wg sync.WaitGroup
		wg.Add(len(idsToGet))
		for _, id := range idsToGet {
			// These will batch together
			go func() {
				defer wg.Done()
				usersDB.Find(ctx, id) // nolint:errcheck
			}()
		}
		wg.Wait()
		done <- struct{}{}
	}()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("timed out")
			return
		case <-done:
			fmt.Println("done")
			return
		}
	}
	// Output: batch: [1 2 3]
	// done
}
