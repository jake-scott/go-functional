/*
Package functional provides some functional programming constructs for Golang.
*/
package functional

import (
	"slices"
	"sync"
)

// FilterFunc is a generic function type that takes a single element and
// returns true if it is to be includes or false if the element is to be
// excluded.
//
// Example:
//
//	func findEvenInts(i int) bool {
//	    return i%2 == 0
//	}
type FilterFunc[T any] func(T) bool

// SimpleFilter executes f for each element of list, returning a new slice of
// filtered elements in the same order as list
//
// Example:
//
//	ints := []int{1,2,3,4,5,6,7,8}
//	filtered := Filter(ints, findEvenInts)
func SimpleFilter[T any](list []T, f FilterFunc[T]) []T {
	out := make([]T, 0, len(list))
	for _, item := range list {
		if f(item) {
			out = append(out, item)
		}
	}

	return out
}

type filterOptions struct {
	parallelism uint
}

type FilterOption func(opts *filterOptions)

func FilterOptionParallelism(num uint) FilterOption {
	return func(opts *filterOptions) {
		opts.parallelism = num
	}
}

func Filter[T any](list []T, f FilterFunc[T], opts ...FilterOption) []T {
	fo := filterOptions{
		parallelism: 1,
	}
	for _, opt := range opts {
		opt(&fo)
	}

	if fo.parallelism <= 1 {
		return SimpleFilter(list, f)
	}
	return ParallelFilter(list, fo.parallelism, f)
}

// ParallelFilter is functionally the same as Filter(), but executes in
// up to maxParallel go routines in a pool.
//
// The pool pattern was chosen as it is much faster and incurs fewer
// allocations than an alternative using a go-routine per item with
// parallelism controlled by a semaphone.
//
// Note that the overhead of setting up the go routines and the channel
// communications is quite high.  Users are advised to benchmark whether this
// overhead is justified compared to the simpler single threaded Filter().
//
// For small lists and/or fast filter functions, ParallelFilter is expected
// to be orders of magnitude slower than the single threaded version.  It
// is designed to operate over either very large (> 1m) lists or with slow
// filter functions (eg. computationally intensive or those that involve
// network calls).
func ParallelFilter[T any](list []T, maxParallel uint, f FilterFunc[T]) []T {
	numParallel := min(uint(len(list)), maxParallel)

	// the filter go-routines need to know the ID (index) of each item they are
	// filtering to report back to the main thread
	type item struct {
		idx  uint
		item T
	}
	chWr := make(chan item) // main -> worker (query)
	chRd := make(chan uint) // worker -> main (result)

	// (1) Write the items to the main -> worker channel in a separate thread
	// of execution.  We don't need to wait for this to be done as we can
	// tell by way of chWr being closed
	go func() {
		for i, val := range list {
			chWr <- item{uint(i), val}
		}

		close(chWr)
	}()

	// (2) Start worker go-routines.  These read items from chWr until that
	// channel is closed by the producer go-routine (1) above.
	wg := sync.WaitGroup{}
	for i := numParallel; i > 0; i-- {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for item := range chWr {
				if f(item.item) {
					chRd <- item.idx
				}
			}
		}()
	}

	// (3) Wait for the workers in a separate go-routine and close the result
	// channel once they are all done
	go func() {
		wg.Wait()
		close(chRd)
	}()

	// Back in the main thread, read results until the result channel has been
	// closed by (3)
	indexes := make([]uint, 0, len(list))
	for i := range chRd {
		indexes = append(indexes, i)
	}

	// sort the indexes so the output is in the same order as the input
	slices.Sort(indexes)

	// and make the output slice
	output := make([]T, len(indexes))
	for j, i := range indexes {
		output[j] = list[i]
	}

	return output
}
func ParallelFilterX[T any](list []T, maxParallel uint, f FilterFunc[T]) []T {
	numParallel := min(uint(len(list)), maxParallel)

	// the filter go-routines need to know the ID (index) of each item they are
	// filtering to report back to the main thread
	type item struct {
		idx  uint
		item T
	}
	chWr := make(chan item) // main -> worker (query)
	chRd := make(chan uint) // worker -> main (result)

	// (1) Write the items to the main -> worker channel in a separate thread
	// of execution.  We don't need to wait for this to be done as we can
	// tell by way of chWr being closed
	go func() {
		for i, val := range list {
			chWr <- item{uint(i), val}
		}

		close(chWr)
	}()

	// (2) Start worker go-routines.  These read items from chWr until that
	// channel is closed by the producer go-routine (1) above.
	wg := sync.WaitGroup{}
	for i := numParallel; i > 0; i-- {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for item := range chWr {
				if f(item.item) {
					chRd <- item.idx
				}
			}
		}()
	}

	// (3) Wait for the workers in a separate go-routine and close the result
	// channel once they are all done
	go func() {
		wg.Wait()
		close(chRd)
	}()

	// Back in the main thread, read results until the result channel has been
	// closed by (3)
	indexes := make([]uint, 0, len(list))
	for i := range chRd {
		indexes = append(indexes, i)
	}

	// sort the indexes so the output is in the same order as the input
	slices.Sort(indexes)

	// and make the output slice
	output := make([]T, len(indexes))
	for j, i := range indexes {
		output[j] = list[i]
	}

	return output
}

// MapFunc is a generic function that takes a single element and returns
// a single transformed element of the same type.
//
// Example:
//
//	func domainName(s string) string {
//	    return strings.SplitN(s, "@",2)[1]
//	}
type MapFunc[T any] func(T) T

// Map transforms each element of list using f and returns the list of
// transformed elements.
//
// Example:
//
//	words := {"these", "are", "all", "lower"}
//	upper := Map(words, strings.ToUpper)
func Map[T any](list []T, f MapFunc[T]) []T {
	out := make([]T, len(list))
	for i, item := range list {
		out[i] = f(item)
	}

	return out
}

// ReduceFunc is a generic function that takes two arguments and reduces them
// to a single value of the same type
type ReduceFunc[T any] func(T, T) T

func Reduce[T any](list []T, init T, f ReduceFunc[T]) T {
	cur := init
	for _, item := range list {
		cur = f(cur, item)
	}

	return cur
}
