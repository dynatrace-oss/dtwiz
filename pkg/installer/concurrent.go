package installer

import (
	"errors"
	"sync"
)

// RunConcurrently runs every fn in its own goroutine, waits for all of them to
// finish, and returns every non-nil error joined together (nil if all succeeded).
// Each fn is expected to capture its result into a variable owned by the
// caller before returning.
func RunConcurrently(fns ...func() error) error {
	errs := make([]error, len(fns))
	var wg sync.WaitGroup
	wg.Add(len(fns))
	for i, fn := range fns {
		go func(i int, fn func() error) {
			defer wg.Done()
			errs[i] = fn()
		}(i, fn)
	}
	wg.Wait()
	return errors.Join(errs...)
}
