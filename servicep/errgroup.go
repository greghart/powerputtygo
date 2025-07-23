package servicep

import (
	"fmt"
	"log/slog"
)

type ErrGroup interface {
	Go(func() error)
}

func ErrGroupValuer[V any](eg ErrGroup, fn func() (V, error)) <-chan V {
	ch := make(chan V, 1)
	eg.Go(func() error {
		defer close(ch)
		result, err := fn()
		if err != nil {
			slog.Error(fmt.Sprintf("error in errgroup function: %v", err))
			return err
		}
		ch <- result
		return nil
	})
	return ch
}
