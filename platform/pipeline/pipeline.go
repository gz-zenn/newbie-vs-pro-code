// Package pipeline provides a generic bounded worker pool with backpressure.
package pipeline

import (
	"context"
	"sync"
)

// Job fetches a single item of type T for a given key.
type Job[T any] func(ctx context.Context, key string) (T, error)

// Result pairs a key with its outcome.
type Result[T any] struct {
	Key   string
	Value T
	Err   error
}

// Run processes keys through job with bounded concurrency, streaming
// results back over a channel as they complete (backpressure: the
// channel is unbuffered, so producers block until a consumer reads).
func Run[T any](ctx context.Context, keys []string, concurrency int, job Job[T]) <-chan Result[T] {
	out := make(chan Result[T])
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	go func() {
		defer close(out)
		for _, k := range keys {
			select {
			case <-ctx.Done():
				wg.Wait() // let in-flight workers finish before closing out
				return
			case sem <- struct{}{}:
			}
			wg.Add(1)
			go func(key string) {
				defer wg.Done()
				defer func() { <-sem }()
				v, err := job(ctx, key)
				select {
				case out <- Result[T]{Key: key, Value: v, Err: err}:
				case <-ctx.Done():
				}
			}(k)
		}
		wg.Wait()
	}()

	return out
}