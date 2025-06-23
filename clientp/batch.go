package clientp

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BulkResolver is a function that can resolve a batch of IDs into a map of id to results.
// Note that R does not need to be ID-able, we just need *something* that's able to distinguish
// batch requests<->results mapping.
type BulkResolver[ID comparable, R any] func(ctx context.Context, ids []ID) (map[ID]R, error)

// batchRequest is what's fed into our requests channel when a request is made.
// Each request includes a channel for receiving the result on, and a context to run against
// By convention, the first context of a batch is used for the bulk resolver.
type batchRequest[ID comparable, R any] struct {
	Ctx  context.Context
	ID   ID
	Chan chan batchResult[ID, R]
}

// batchResult is what's fed back on a requests' channel once the item has been resolved.
// If consumer wants, they can put a fancy error value that tracks errors by ID, etc.
type batchResult[ID comparable, R any] struct {
	res *R
	err error
}

// Batch will transparently batch multiple ID-able requests into a single batch.
type Batch[ID comparable, R any] struct {
	requests chan batchRequest[ID, R]
	resolver BulkResolver[ID, R]
	timeout  time.Duration
	stopCh   chan any
	once     sync.Once
	// TODO: Max batch size parameter
}

type BatchOptions struct {
	Timeout time.Duration
}

func NewBatch[ID comparable, R any](resolver BulkResolver[ID, R], _opts ...BatchOptions) *Batch[ID, R] {
	// Default options
	opts := BatchOptions{}
	if len(_opts) > 0 {
		opts = _opts[0]
	}
	if opts.Timeout == 0 {
		opts.Timeout = 500 * time.Millisecond
	}

	return &Batch[ID, R]{
		requests: make(chan batchRequest[ID, R]),
		resolver: resolver,
		timeout:  opts.Timeout,
		stopCh:   make(chan any),
	}
}

func (bc *Batch[ID, R]) Stop() {
	close(bc.requests)
}

// Start the batching service.
// Note if you forget this, requests will spin forever!
func (bc *Batch[ID, R]) Start() {
	bc.once.Do(func() {
		go func() {
			ticker := time.NewTicker(bc.timeout)
			defer ticker.Stop()

			reqs := []batchRequest[ID, R]{}
			defer func() {
				for _, req := range reqs {
					close(req.Chan)
				}
			}()

			for {
				select {
				case req, ok := <-bc.requests:
					// add request
					if !ok {
						return
					}
					reqs = append(reqs, req)
				case <-ticker.C:
					// process
					if len(reqs) == 0 {
						continue
					}

					ids := make([]ID, 0, len(reqs))
					for _, req := range reqs {
						ids = append(ids, req.ID)
					}
					results, err := bc.resolver(reqs[0].Ctx, ids)

					// Respond to every waiter, even if we didn't get a result
					for _, req := range reqs {
						var resPtr *R
						if res, ok := results[req.ID]; ok {
							resPtr = &res
						}
						req.Chan <- batchResult[ID, R]{
							res: resPtr,
							err: err,
						}
					}
					reqs = []batchRequest[ID, R]{}
				}
			}
		}()
	})
}

func (c *Batch[ID, R]) Execute(ctx context.Context, id ID) (*R, error) {
	ch := make(chan batchResult[ID, R])

	// Wait
	c.requests <- batchRequest[ID, R]{
		ID:   id,
		Chan: ch,
		Ctx:  ctx,
	}
	res, ok := <-ch
	if !ok {
		return nil, fmt.Errorf("batch closed before closure")
	}
	return res.res, res.err
}
