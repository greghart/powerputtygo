package clientp

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestBatchClient(t *testing.T) {
	expected := map[int64]*Result{
		1: {1, "Hello"},
		2: {2, "World"},
		3: {3, "NinjaOne"},
		4: nil,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Successful gets
	t.Run("successful batch", func(t *testing.T) {
		db := newFakeDB()
		bc := newBatchFakeDBClient(db)
		bc.Start()
		defer bc.Stop()

		idsToGet := []int64{1, 2, 3, 3, 4}

		var wg sync.WaitGroup
		wg.Add(len(idsToGet))
		for _, id := range idsToGet {
			go func() {
				defer wg.Done()
				// db.Find(id) would do separate fetches for all the above!
				res, err := bc.Find(ctx, id)
				if err != nil {
					t.Errorf("request %v failed with", err)
					return
				}
				if !cmp.Equal(expected[id], res) {
					t.Errorf("request %v returned unexpected result:\n%s", id, cmp.Diff(expected[id], res))
				}
			}()
		}
		wg.Wait()
		expectedCalls := []string{"WhereID([1 2 3 3 4])"}
		if !cmp.Equal(db.calls, expectedCalls) {
			t.Errorf("db calls did not match expected:\n%s", cmp.Diff(db.calls, expectedCalls))
		}
	})
	t.Run("timeout non-batch", func(t *testing.T) {
		db := newFakeDB()
		bc := newBatchFakeDBClient(db)
		bc.Start()
		defer bc.Stop()

		go func() {
			res, err := bc.Find(ctx, 1)
			if err != nil {
				t.Errorf("request 1 failed with %v", err)
			}
			if !cmp.Equal(expected[1], res) {
				t.Errorf("request 1 returned unexpected result:\n%s", cmp.Diff(expected[1], res))
			}
		}()
		time.Sleep(50 * time.Millisecond) // Wait for timeout
		go func() {
			res, err := bc.Find(ctx, 2)
			if err != nil {
				t.Errorf("request 2 failed with %v", err)
			}
			if !cmp.Equal(expected[2], res) {
				t.Errorf("request 2 returned unexpected result:\n%s", cmp.Diff(expected[2], res))
			}
		}()
		time.Sleep(50 * time.Millisecond) // Wait for timeout

		expectedCalls := []string{"WhereID([1])", "WhereID([2])"}
		if !cmp.Equal(db.calls, expectedCalls) {
			t.Errorf("db calls did not match expected:\n%s", cmp.Diff(db.calls, expectedCalls))
		}
	})

	t.Run("failure batch from stopped batch client", func(t *testing.T) {
		db := newFakeDB()
		bc := newBatchFakeDBClient(db)
		bc.Start()

		var wg sync.WaitGroup
		wg.Add(3)
		for _, id := range []int64{1, 2, 3} {
			go func() {
				defer wg.Done()
				_, err := bc.Find(ctx, id)
				if err == nil {
					t.Errorf("request %v did not fail!", id)
				}
			}()
		}
		go func() {
			time.Sleep(2 * time.Millisecond)
			bc.Stop()
		}()
		wg.Wait()
	})
}

////////////////////////////////////////////////////////////////////////////////

type batchFakeDBClient struct {
	db *fakeDB
	*Batch[int64, Result]
}

func newBatchFakeDBClient(db *fakeDB) *batchFakeDBClient {
	bc := NewBatch(
		func(ctx context.Context, ids []int64) (map[int64]Result, error) {
			results, err := db.WhereID(ids)
			if err != nil {
				return nil, err
			}

			out := make(map[int64]Result, len(results))
			for _, res := range results {
				out[res.ID] = res
			}
			return out, nil
		},
		BatchOptions{Timeout: 10 * time.Millisecond},
	)

	return &batchFakeDBClient{
		Batch: bc,
		db:    db,
	}
}

func (bc *batchFakeDBClient) Find(ctx context.Context, id int64) (*Result, error) {
	return bc.Execute(ctx, id)
}

////////////////////////////////////////////////////////////////////////////////

type Result struct {
	ID   int64
	Data string
}

////////////////////////////////////////////////////////////////////////////////

// fakeDB to simulate batchable processes
type fakeDB struct {
	results []Result
	calls   []string
}

func newFakeDB() *fakeDB {
	return &fakeDB{
		results: []Result{
			{1, "Hello"},
			{2, "World"},
			{3, "NinjaOne"},
		},
		calls: []string{},
	}
}

func (db *fakeDB) WhereID(ids []int64) ([]Result, error) {
	slices.Sort(ids)
	db.calls = append(db.calls, fmt.Sprintf("WhereID(%v)", ids))
	// Pretend this is a query
	out := []Result{}
	for _, v := range db.results {
		if slices.Contains(ids, v.ID) {
			out = append(out, v)
		}
	}
	return out, nil
}

func (db *fakeDB) Find(id int64) (*Result, error) {
	db.calls = append(db.calls, fmt.Sprintf("Find(%v)", id))
	// Pretend this is a query
	for _, v := range db.results {
		if v.ID == id {
			return &v, nil
		}
	}
	return nil, nil
}
