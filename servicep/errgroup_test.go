package servicep

import (
	"context"
	"net/http"
	"testing"

	"golang.org/x/sync/errgroup"
)

func TestErrGroupValuer_httpExample(t *testing.T) {
	t.Skip("hits internet")
	fetchURL := func(ctx context.Context, url string) func() (int, error) {
		return func() (int, error) {
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) // nolint:errcheck
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				return 0, err
			}
			return res.StatusCode, nil
		}
	}

	g, ctx := errgroup.WithContext(context.Background())
	one := ErrGroupValuer(g, fetchURL(ctx, "https://google.com"))
	two := ErrGroupValuer(g, fetchURL(ctx, "https://microsoft.com"))
	three := ErrGroupValuer(g, fetchURL(ctx, "https://spotify.com"))
	err := g.Wait()
	if err != nil {
		t.Fatalf("errgroup failed: %v", err)
	}
	resOne := <-one
	resTwo := <-two
	resThree := <-three
	t.Logf("Results: %d, %d, %d", resOne, resTwo, resThree)
}
