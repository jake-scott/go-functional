package functional

import (
	"cmp"
	"slices"
	"sync"

	"github.com/jake-scott/go-functional/iter/channel"
	"github.com/jake-scott/go-functional/iter/slice"
)

// FilterFunc is a generic function type that takes a single element and
// returns true if it is to be included or false if the element is to be
// excluded from the result set.
//
// Example:
//
//	func findEvenInts(i int) bool {
//	    return i%2 == 0
//	}
type FilterFunc[T any] func(T) bool

// Filter is the non-OO version of Stage.Filter().
func Filter[T any](s *Stage[T], f FilterFunc[T], opts ...StageOption) *Stage[T] {
	return s.Filter(f, opts...)
}

// Filter processes this stage's input elements by calling f for each element
// and returns a new stage that will process all the elements where f(e)
// is true.
//
// If this stage is configured to process in batch, Filter returns after all
// the input elements have been processed;  those elements are passed to the
// next stage as a slice.
//
// If this stage is configured to stream, Filter returns immediately after
// launching a go-routine to process the elements in the background.  The
// next stage reads from a channel that the processing goroutine writes its
// results to as they are processed.
func (s *Stage[T]) Filter(f FilterFunc[T], opts ...StageOption) *Stage[T] {
	// opts for this Filter are the stage options overridden by the filter
	// specific options passed to this call
	merged := *s
	merged.opts.processOptions(opts...)

	t := merged.tracer("Filter")
	defer t.End()

	// Run a batch or streaming filter.  The streaming filter will return
	// immediately.
	var i Iterator[T]
	switch merged.opts.stageType {
	case BatchStage:
		i = merged.filterBatch(t, f)
	case StreamingStage:
		i = merged.filterStreaming(t, f)
	}

	return s.nextStage(i, opts...)
}

func (s *Stage[T]) filterBatch(t Tracer, f FilterFunc[T]) Iterator[T] {
	tBatch := t.SubTracer("batch")
	defer tBatch.End()

	if sh, ok := s.i.(Size[T]); ok {
		s.opts.sizeHint = sh.Size()
	}

	// handle parallel filters separately..
	if s.opts.maxParallelism > 1 {
		return s.parallelBatchFilter(tBatch, f)
	}

	t.Msg("Sequential processing")

	out := make([]T, 0, s.opts.sizeHint)
	for s.i.Next(s.opts.ctx) {
		item := s.i.Get(s.opts.ctx)
		if f(item) {
			out = append(out, item)
		}
	}

	i := slice.New(out)

	return &i
}

func (s *Stage[T]) parallelBatchFilter(t Tracer, f FilterFunc[T]) Iterator[T] {
	var tParallel Tracer

	var output []T
	if s.opts.preserveOrder {
		tParallel = t.SubTracer("parallel, ordered")

		// when preserving order, we call parallelBatchFilterProcessor[T, item[T]],
		// so we receive a []item[T] as a return value
		results := parallelBatchFilterProcessor(s, tParallel, f, orderedWrapper[T], orderedUnwrapper[T])

		// sort by the index of each item[T]
		slices.SortFunc(results, func(a, b item[T]) int {
			return cmp.Compare(a.idx, b.idx)
		})

		// pull the values from the item[T] results
		output = make([]T, len(results))
		for i, v := range results {
			output[i] = v.item
		}
	} else {
		// when not preserving order, we call parallelBatchFilterProcessor[T, T],
		// and so we receive a []T as a return value
		tParallel = t.SubTracer("parallel, unordered")
		output = parallelBatchFilterProcessor(s, tParallel, f, unorderedWrapper[T], unorderedUnwrapper[T])
	}

	i := slice.New(output)
	tParallel.End()
	return &i
}

// Filter the stage's input items.  Returns items wrapped by wrapper which
// could include the original index of the input items for use in the caller to
// sort the result.
func parallelBatchFilterProcessor[T any, TW any](s *Stage[T], t Tracer, f FilterFunc[T], wrapper wrapItemFunc[T, TW], unwrapper unwrapItemFunc[TW, T]) []TW {
	numParallel := min(s.opts.sizeHint, s.opts.maxParallelism)

	t = t.SubTracer("parallelization=%d", numParallel)
	defer t.End()

	chIn := make(chan TW)  // main -> worker (query)
	chOut := make(chan TW) // worker -> main (result)

	// (1) Write the items to the main -> worker channel in a separate thread
	// of execution.  We don't need to wait for this to be done as we can
	// tell by way of chWr being closed
	go func() {
		t := t.SubTracer("reader")
		// if anything goes wrong, close chWr to avoid goroutine leaks
		defer func() {
			closeChanIfOpen(chIn)
		}()

		i := 0
		for s.i.Next(s.opts.ctx) {
			item := wrapper(uint(i), s.i.Get(s.opts.ctx))
			chIn <- item
			i++
		}

		t.End()
	}()

	// (2) Start worker go-routines.  These read items from chWr until that
	// channel is closed by the producer go-routine (1) above.
	wg := sync.WaitGroup{}
	for i := numParallel; i > 0; i-- {
		wg.Add(1)

		i := i
		go func() {
			t := t.SubTracer("processor %d", i)

			defer wg.Done()
			defer t.End()

		readLoop:
			for item := range chIn {
				if f(unwrapper(item)) {
					select {
					case chOut <- item:
					case <-s.opts.ctx.Done():
						t.Msg("Cancelled")
						break readLoop
					}
				}
			}
		}()
	}

	// (3) Wait for the workers in a separate go-routine and close the result
	// channel once they are all done
	go func() {
		t := t.SubTracer("wait for processors")

		wg.Wait()
		close(chOut)

		t.End()
	}()

	// Back in the main thread, read results until the result channel has been
	// closed by (3)
	items := make([]TW, 0, s.opts.sizeHint)
	for i := range chOut {
		items = append(items, i)
	}

	return items
}

func closeChanIfOpen[T any](ch chan T) {
	ok := true
	select {
	case _, ok = <-ch:
	default:
	}
	if ok {
		defer func() {
			_ = recover()
		}()

		close(ch)
	}
}

func (s *Stage[T]) filterStreaming(t Tracer, f FilterFunc[T]) Iterator[T] {
	if sh, ok := s.i.(Size[T]); ok {
		s.opts.sizeHint = sh.Size()
	}

	// handle parallel filters separately..
	if s.opts.maxParallelism > 1 {
		return s.parallelStreamingFilter(t, f)
	}

	// otherwise just run a simple serial filter
	t = t.SubTracer("streaming, sequential")

	ch := make(chan T)

	go func() {
		t := s.tracer("processor")
		defer t.End()

	readLoop:
		for s.i.Next(s.opts.ctx) {
			item := s.i.Get(s.opts.ctx)
			if f(item) {
				select {
				case ch <- item:
				case <-s.opts.ctx.Done():
					t.Msg("Cancelled")
					break readLoop
				}
			}
		}
		close(ch)
	}()

	i := channel.New(ch)
	t.End()
	return &i
}

func (s *Stage[T]) parallelStreamingFilter(t Tracer, f FilterFunc[T]) Iterator[T] {
	numParallel := min(s.opts.sizeHint, s.opts.maxParallelism)

	t = t.SubTracer("streaming, parallel=%d", numParallel)
	defer t.End()

	chIn := make(chan T)  // main -> worker (query)
	chOut := make(chan T) // worker -> next stage (result)

	// (1) Write the items to the main -> worker channel in a separate thread
	// of execution.  We don't need to wait for this to be done as we can
	// tell by way of chWr being closed
	go func() {
		t := t.SubTracer("reader")
		// if anything goes wrong, close chIn to avoid goroutine leaks
		defer func() {
			closeChanIfOpen(chIn)
		}()

		for s.i.Next(s.opts.ctx) {
			chIn <- s.i.Get(s.opts.ctx)
		}

		t.End()
	}()

	// (2) Start worker go-routines.  These read items from chIn until that
	// channel is closed by the producer go-routine (1) above.
	wg := sync.WaitGroup{}
	for i := numParallel; i > 0; i-- {
		wg.Add(1)

		i := i
		go func() {
			t := t.SubTracer("processor %d", i)

			defer wg.Done()
			defer t.End()

		workerLoop:
			for item := range chIn {
				if f(item) {
					select {
					case chOut <- item:
					case <-s.opts.ctx.Done():
						t.Msg("Cancelled")
						break workerLoop
					}
				}
			}
		}()
	}

	// (3) Wait for the workers in a separate go-routine and close the result
	// channel once they are all done
	go func() {
		t := t.SubTracer("wait for processors")

		wg.Wait()
		close(chOut)

		t.End()
	}()

	i := channel.New(chOut)
	return &i
}
