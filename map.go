package functional

import (
	"cmp"
	"slices"

	"github.com/jake-scott/go-functional/iter/channel"
	"github.com/jake-scott/go-functional/iter/slice"
)

// MapFunc is a generic function that takes a single element and returns
// a single transformed element.
//
// Example:
//
//	func ipAddress(host string) (net.IP, error) {
//	    return net.LookupIp(host)
//	}
type MapFunc[T any, M any] func(T) (M, error)

// Map processes the stage's input elements by calling m for each element,
// returning a new stage containing the same number of elements, mapped to
// new values of the same type.
//
// If the map function returns values of a different type to the input values,
// the non-OO version of Map() must be used instead.
//
// If the stage is configured to process in batch, Map returns after all the
// input elements have been processed;  those elements are passed to the next
// stage as a slice.
//
// If the stage is configued to stream, Map returns immediately after
// launching go-routines to process the elements in the background.  The
// returned stage reads from a channel that the processing goroutine writes
// its result to as they are processed.
func (s *Stage[T]) Map(m MapFunc[T, T], opts ...StageOption) *Stage[T] {
	return Map(s, m, opts...)
}

// Map is the non-OO version of Stage.Map().  It must be used in the case
// where the map function returns items of a different type than the input
// elements, due to limitations of Golang's generic syntax.
func Map[T, M any](s *Stage[T], m MapFunc[T, M], opts ...StageOption) *Stage[M] {
	// opts for this Map are the stage options overridden by the Map
	// specific options passed to this call
	merged := *s
	merged.opts.processOptions(opts...)

	t := merged.tracer("Map")
	defer t.end()

	var i Iterator[M]
	switch merged.opts.stageType {
	case BatchStage:
		i = mapBatch(t, &merged, m)
	case StreamingStage:
		i = mapStreaming(t, &merged, m)
	}

	return nextStage(s, i, opts...)
}

// T: input type;  M: mapped type
func mapBatch[T, M any](t tracer, s *Stage[T], m MapFunc[T, M]) Iterator[M] {
	t = t.subTracer("batch")
	defer t.end()

	if sh, ok := s.i.(Size[T]); ok {
		s.opts.sizeHint = sh.Size()
	}

	// handle parallel maps separately..
	if s.opts.maxParallelism > 1 {
		return mapBatchParallel(t, s, m)
	}

	t.msg("Sequential processing")

	out := make([]M, 0, s.opts.sizeHint)

mapLoop:
	for s.i.Next(s.opts.ctx) {
		item := s.i.Get()
		newItem, err := m(item)
		switch {
		case err != nil:
			if !s.opts.onError(ErrorContextMapFunction, err) {
				t.msg("map done due to error: %s", err)
				break mapLoop
			}
		default:
			out = append(out, newItem)
		}
	}

	i := slice.New(out)
	return &i
}

// T: input type;  M: mapped type
func mapBatchParallel[T, M any](t tracer, s *Stage[T], m MapFunc[T, M]) Iterator[M] {
	var tParallel tracer

	var output []M
	if s.opts.preserveOrder {
		tParallel = t.subTracer("parallel, ordered")

		results := mapBatchParallelProcessor(s, tParallel, m, orderedWrapper[T], orderedUnwrapper[T], orderedSwitcher[T, M])

		// sort by the index of each item[M]
		slices.SortFunc(results, func(a, b item[M]) int {
			return cmp.Compare(a.idx, b.idx)
		})

		// pull the values from the item[T] results
		output = make([]M, len(results))
		for i, v := range results {
			output[i] = v.item
		}
	} else {
		tParallel = t.subTracer("parallel, unordered")
		output = mapBatchParallelProcessor(s, tParallel, m, unorderedWrapper[T], unorderedUnwrapper[T], unorderedSwitcher[T, M])
	}

	i := slice.New(output)
	tParallel.end()
	return &i
}

// T: input type;   TW: wrapped input type;   M: mapped type;   MW: wrapped map type
func mapBatchParallelProcessor[T, TW, M, MW any](s *Stage[T], t tracer, m MapFunc[T, M],
	wrapper wrapItemFunc[T, TW], unwrapper unwrapItemFunc[TW, T], switcher switcherFunc[TW, M, MW]) []MW {

	numParallel := min(s.opts.sizeHint, s.opts.maxParallelism)

	t = t.subTracer("parallelization=%d", numParallel)
	defer t.end()

	chOut := parallelProcessor(s.opts, numParallel, s.i, t,
		// write wrapped input values to the query channel
		func(i uint, t T, ch chan TW) {
			item := wrapper(i, t)
			ch <- item
		},

		// read wrapped input values, write mapped wrapped output values to
		// the output channel
		func(item TW, ch chan MW) error {
			// Make a MW item from the TW item using a map function
			mappedValue, err := m(unwrapper(item))

			if err != nil {
				return err
			}

			itemOut := switcher(item, mappedValue)
			select {
			case ch <- itemOut:
			case <-s.opts.ctx.Done():
				return s.opts.ctx.Err()
			}

			return nil
		})

	// Back in the main thread, read results until the result channel has been
	// closed by (3)
	items := make([]MW, 0, s.opts.sizeHint)
	for i := range chOut {
		items = append(items, i)
	}

	return items
}

// T: input type;  M: mapped type
func mapStreaming[T, M any](t tracer, s *Stage[T], m MapFunc[T, M]) Iterator[M] {
	if sh, ok := s.i.(Size[T]); ok {
		s.opts.sizeHint = sh.Size()
	}

	// handle parallel maps separately..
	if s.opts.maxParallelism > 1 {
		return mapStreamingParallel(t, s, m)
	}

	// otherwise just run a simple serial map
	t = t.subTracer("streaming, sequential")

	ch := make(chan M)

	go func() {
		t := s.tracer("Map, streaming, sequential background")
		defer t.end()

	readLoop:
		for s.i.Next(s.opts.ctx) {
			item := s.i.Get()
			mapped, err := m(item)

			switch {
			case err != nil:
				if !s.opts.onError(ErrorContextFilterFunction, err) {
					t.msg("map done due to error: %s", err)
					break readLoop
				}
			default:
				select {
				case ch <- mapped:
				case <-s.opts.ctx.Done():
					t.msg("Cancelled")
					break readLoop
				}
			}
		}
		close(ch)
	}()

	i := channel.New(ch)
	t.end()
	return &i
}

func mapStreamingParallel[T, M any](t tracer, s *Stage[T], m MapFunc[T, M]) Iterator[M] {
	numParallel := min(s.opts.sizeHint, s.opts.maxParallelism)

	t = t.subTracer("streaming, parallel=%d", numParallel)
	defer t.end()

	chOut := parallelProcessor(s.opts, numParallel, s.i, t,
		func(i uint, t T, ch chan T) {
			ch <- t
		},

		func(item T, ch chan M) error {
			mapped, err := m(item)
			if err != nil {
				return err
			}

			select {
			case ch <- mapped:
			case <-s.opts.ctx.Done():
				return s.opts.ctx.Err()
			}

			return nil
		})

	i := channel.New(chOut)
	return &i
}
