package functional

import (
	"cmp"
	"slices"

	"github.com/jake-scott/go-functional/iter/channel"
	"github.com/jake-scott/go-functional/iter/slice"
)

// FilterFunc is a generic function type that takes a single element and
// returns true if it is to be included or false if the element is to be
// excluded from the result set.
//
// If an error is returned, it is passed to the stage's error handler
// function which may elect to continue or abort processing.
//
// Example:
//
//	func findEvenInts(i int) (bool, error) {
//	    return i%2 == 0, nil
//	}
type FilterFunc[T any] func(T) (bool, error)

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
	defer t.end()

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

func (s *Stage[T]) filterBatch(t tracer, f FilterFunc[T]) Iterator[T] {
	tBatch := t.subTracer("batch")
	defer tBatch.end()

	if sh, ok := s.i.(Size[T]); ok {
		s.opts.sizeHint = sh.Size()
	}

	// handle parallel filters separately..
	if s.opts.maxParallelism > 1 {
		return s.parallelBatchFilter(tBatch, f)
	}

	t.msg("Sequential processing")

	out := make([]T, 0, s.opts.sizeHint)

filterLoop:
	for s.i.Next(s.opts.ctx) {
		item := s.i.Get()
		keep, err := f(item)
		switch {
		case err != nil:
			if !s.opts.onError(ErrorContextFilterFunction, err) {
				t.msg("filter done due to error: %s", err)
				break filterLoop
			}
		case keep:
			out = append(out, item)
		}
	}

	if s.i.Error() != nil {
		// if there is an iterator read error ..
		if !s.opts.onError(ErrorContextItertator, s.i.Error()) {
			// clear the output slice if we are told not to continue
			out = []T{}
		}
	}

	i := slice.New(out)

	return &i
}

func (s *Stage[T]) parallelBatchFilter(t tracer, f FilterFunc[T]) Iterator[T] {
	var tParallel tracer

	var output []T
	if s.opts.preserveOrder {
		tParallel = t.subTracer("parallel, ordered")

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
		tParallel = t.subTracer("parallel, unordered")
		output = parallelBatchFilterProcessor(s, tParallel, f, unorderedWrapper[T], unorderedUnwrapper[T])
	}

	i := slice.New(output)
	tParallel.end()
	return &i
}

// Filter the stage's input items.  Returns items wrapped by wrapper which
// could include the original index of the input items for use in the caller to
// sort the result.
//
// T: input type;   TW: wrapped input type
func parallelBatchFilterProcessor[T any, TW any](s *Stage[T], t tracer, f FilterFunc[T],
	wrapper wrapItemFunc[T, TW], unwrapper unwrapItemFunc[TW, T]) []TW {

	numParallel := min(s.opts.sizeHint, s.opts.maxParallelism)

	t = t.subTracer("parallelization=%d", numParallel)
	defer t.end()

	// MW is inferred to be the same as TW for a filter..
	chOut := parallelProcessor(s.opts, numParallel, s.i, t,
		// write wrapped input values to the query channel
		func(i uint, t T, ch chan TW) {
			item := wrapper(i, t)
			ch <- item
		},

		// read wrapped values, write to the output channel if f() == true
		func(item TW, ch chan TW) error {
			keep, err := f(unwrapper(item))

			if err != nil {
				return err
			}

			if keep {
				select {
				case ch <- item:
				case <-s.opts.ctx.Done():
					return s.opts.ctx.Err()
				}
			}
			return nil
		})

	if s.i.Error() != nil {
		// if there is an iterator read error ..
		if !s.opts.onError(ErrorContextItertator, s.i.Error()) {
			// return no items if we are told not to continue
			return []TW{}
		}
	}

	// Back in the main thread, read results until the result channel has been
	// closed by a go-routine started by parallelProcessor()
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

func (s *Stage[T]) filterStreaming(t tracer, f FilterFunc[T]) Iterator[T] {
	if sh, ok := s.i.(Size[T]); ok {
		s.opts.sizeHint = sh.Size()
	}

	// handle parallel filters separately..
	if s.opts.maxParallelism > 1 {
		return s.parallelStreamingFilter(t, f)
	}

	// otherwise just run a simple serial filter
	t = t.subTracer("streaming, sequential")

	ch := make(chan T)

	go func() {
		t := s.tracer("processor")
		defer t.end()

	readLoop:
		for s.i.Next(s.opts.ctx) {
			item := s.i.Get()
			keep, err := f(item)

			switch {
			case err != nil:
				if !s.opts.onError(ErrorContextFilterFunction, err) {
					t.msg("filter done due to error: %s", err)
					break readLoop
				}
			case keep:
				select {
				case ch <- item:
				case <-s.opts.ctx.Done():
					t.msg("Cancelled")
					break readLoop
				}
			}
		}
		close(ch)

		// if there is an iterator read error, report it even though we
		// can't abort the next stage; at least we can stop sending items
		// to it by way of having closed ch
		if s.i.Error() != nil {
			s.opts.onError(ErrorContextItertator, s.i.Error())
		}
	}()

	i := channel.New(ch)
	t.end()
	return &i
}

func (s *Stage[T]) parallelStreamingFilter(t tracer, f FilterFunc[T]) Iterator[T] {
	numParallel := min(s.opts.sizeHint, s.opts.maxParallelism)

	t = t.subTracer("streaming, parallel=%d", numParallel)
	defer t.end()

	chOut := parallelProcessor(s.opts, numParallel, s.i, t,
		// write input values to the query channel
		func(i uint, t T, ch chan T) {
			ch <- t
		},

		// read values from the query channel, write to the output channel if f() == true
		func(t T, ch chan T) error {
			keep, err := f(t)
			if err != nil {
				return err
			}
			if keep {
				select {
				case ch <- t:
				case <-s.opts.ctx.Done():
					return s.opts.ctx.Err()
				}
			}
			return nil
		})

	i := channel.New(chOut)
	return &i
}
