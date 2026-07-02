package installer

import "sync"

// RunConcurrently runs every fn in its own goroutine, waits for all of them to
// finish, and returns the first non-nil error in argument order (if any).
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
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
