package functional

import (
	"cmp"
	"slices"
	"sync"

	"github.com/jake-scott/go-functional/iter/channel"
	"github.com/jake-scott/go-functional/iter/slice"
)

// MapFunc is a generic function that takes a single element and returns
// a single transformed element.
//
// Example:
//
//	func ipAddress(host string) net.IP {
//	    ip, _ = net.LookupIp(host)
//		return ip
//	}
type MapFunc[T any, M any] func(T) M

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
	defer t.End()

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
func mapBatch[T, M any](t Tracer, s *Stage[T], m MapFunc[T, M]) Iterator[M] {
	t = t.SubTracer("batch")
	defer t.End()

	if sh, ok := s.i.(Size[T]); ok {
		s.opts.sizeHint = sh.Size()
	}

	// handle parallel maps separately..
	if s.opts.maxParallelism > 1 {
		return mapBatchParallel(t, s, m)
	}

	t.Msg("Sequential processing")

	out := make([]M, 0, s.opts.sizeHint)
	for s.i.Next(s.opts.ctx) {
		item := s.i.Get(s.opts.ctx)
		out = append(out, m(item))
	}

	i := slice.New(out)
	return &i
}

// T: input type;  M: mapped type
func mapBatchParallel[T, M any](t Tracer, s *Stage[T], m MapFunc[T, M]) Iterator[M] {
	var tParallel Tracer

	var output []M
	if s.opts.preserveOrder {
		tParallel = t.SubTracer("parallel, ordered")

		results := mapBatchParallelProcessor[T, item[T], M, item[M]](tParallel, s, m, orderedWrapper[T], orderedUnwrapper[T], orderedSwitcher[T, M])

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
		tParallel = t.SubTracer("parallel, unordered")
		output = mapBatchParallelProcessor[T, T, M, M](tParallel, s, m, unorderedWrapper[T], unorderedUnwrapper[T], unorderedSwitcher[T, M])
	}

	i := slice.New(output)
	tParallel.End()
	return &i
}

// T: input type;   TW: wrapped input type;   M: mapped type;   MW: wrapped map type
func mapBatchParallelProcessor[T, TW, M, MW any](t Tracer, s *Stage[T], m MapFunc[T, M], wrapper wrapItemFunc[T, TW], unwrapper unwrapItemFunc[TW, T], switcher switcherFunc[TW, M, MW]) []MW {
	numParallel := min(s.opts.sizeHint, s.opts.maxParallelism)

	t = t.SubTracer("parallelization=%d", numParallel)
	defer t.End()

	chIn := make(chan TW)  // main -> worker (query)
	chOut := make(chan MW) // worker -> main (result)

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

		workerLoop:
			for itemIn := range chIn {
				itemOut := switcher(itemIn, m(unwrapper(itemIn)))
				select {
				case chOut <- itemOut:
				case <-s.opts.ctx.Done():
					t.Msg("Cancelled")
					break workerLoop
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
	items := make([]MW, 0, s.opts.sizeHint)
	for i := range chOut {
		items = append(items, i)
	}

	return items
}

// T: input type;  M: mapped type
func mapStreaming[T, M any](t Tracer, s *Stage[T], m MapFunc[T, M]) Iterator[M] {
	if sh, ok := s.i.(Size[T]); ok {
		s.opts.sizeHint = sh.Size()
	}

	// handle parallel maps separately..
	if s.opts.maxParallelism > 1 {
		return mapStreamingParallel(t, s, m)
	}

	// otherwise just run a simple serial map
	t = t.SubTracer("streaming, sequential")

	ch := make(chan M)

	go func() {
		t := s.tracer("Map, streaming, sequential background")
		defer t.End()

	readLoop:
		for s.i.Next(s.opts.ctx) {
			item := s.i.Get(s.opts.ctx)
			mapped := m(item)
			select {
			case ch <- mapped:
			case <-s.opts.ctx.Done():
				t.Msg("Cancelled")
				break readLoop
			}
		}
		close(ch)
	}()

	i := channel.New(ch)
	t.End()
	return &i
}

func mapStreamingParallel[T, M any](t Tracer, s *Stage[T], m MapFunc[T, M], opts ...StageOption) Iterator[M] {
	numParallel := min(s.opts.sizeHint, s.opts.maxParallelism)

	t = t.SubTracer("streaming, parallel=%d", numParallel)
	defer t.End()

	chIn := make(chan T)  // main -> worker (query)
	chOut := make(chan M) // worker -> next stage (result)

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
				select {
				case chOut <- m(item):
				case <-s.opts.ctx.Done():
					t.Msg("Cancelled")
					break workerLoop
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
