package functional

import (
	"cmp"
	"fmt"
	"slices"
	"sync"
)

type GeneratorType int

const (
	Batch GeneratorType = iota
	Streaming
)

type Generator[T any] struct {
	generatorType  GeneratorType
	maxParallelism uint
	preserveOrder  bool
	inheritOptions bool
	i              Iterator[T]
}

type GeneratorOption[T any] func(g *Generator[T])

func ProcessingType[T any](t GeneratorType) GeneratorOption[T] {
	return func(g *Generator[T]) {
		g.generatorType = t
	}
}

func Parallelism[T any](max uint) GeneratorOption[T] {
	return func(g *Generator[T]) {
		g.maxParallelism = max
	}
}

func PreserveOrder[T any](preserve bool) GeneratorOption[T] {
	return func(g *Generator[T]) {
		g.preserveOrder = preserve
	}
}

func InheritOptions[T any](inherit bool) GeneratorOption[T] {
	return func(g *Generator[T]) {
		g.inheritOptions = inherit
	}
}

func (g *Generator[T]) processOptions(opts ...GeneratorOption[T]) {
	for _, f := range opts {
		f(g)
	}
}

func NewSliceGenerator[T any](s []T, opts ...GeneratorOption[T]) *Generator[T] {
	i := NewSliceIterator(s)
	g := &Generator[T]{
		i: &i,
	}
	g.processOptions(opts...)
	return g
}

func NewChannelGenerator[T any](ch chan T, opts ...GeneratorOption[T]) *Generator[T] {
	i := NewChannelIterator(ch)
	g := &Generator[T]{
		i: &i,
	}
	g.processOptions(opts...)
	return g
}

func (g *Generator[T]) Iterator() Iterator[T] {
	return g.i
}

func (g *Generator[T]) Filter(f FilterFunc[T], opts ...GeneratorOption[T]) *Generator[T] {
	// let us inspect options for this Filter call on their own
	var newOpts Generator[T]
	newOpts.processOptions(opts...)

	// opts for this Filter are those passed in from the previous step
	// overridden by this call's specific options
	merged := *g
	merged.processOptions(opts...)

	var i Iterator[T]
	switch g.generatorType {
	case Batch:
		i = merged.filterBatch(f)
	case Streaming:
		i = merged.filterStreaming(f)
	}

	// the next step's options...
	var nextGenerator Generator[T]

	// .. include those from the previous step if inherit is enabled
	if g.inheritOptions {
		nextGenerator = *g
	}
	// .. possibly overridden by this step's options if those have inherit enabled
	if newOpts.inheritOptions {
		nextGenerator.processOptions(opts...)
	}

	nextGenerator.i = i

	return &nextGenerator
}

func (g *Generator[T]) filterBatch(f FilterFunc[T]) Iterator[T] {

	if g.maxParallelism > 1 {
		if g.preserveOrder {
			return g.parallelBatchFilter(f)
		} else {
			return g.parallelBatchFilterNoOrder(f)
		}
	}

	size := uint(10)
	if sh, ok := g.i.(SizeHint[T]); ok {
		size = sh.Size()
	}

	// not parallel -- just do a simple iterative filter
	out := make([]T, 0, size)
	for g.i.Next() {
		item := g.i.Get()
		if f(item) {
			out = append(out, item)
		}
	}

	i := NewSliceIterator(out)
	return &i
}

func (g *Generator[T]) parallelBatchFilter(f FilterFunc[T]) Iterator[T] {
	size := g.maxParallelism
	if sh, ok := g.i.(SizeHint[T]); ok {
		size = sh.Size()
	}

	numParallel := min(size, g.maxParallelism)

	// the filter go-routines need to know the ID (index) of each item they are
	// filtering to report back to the main thread for output sorting
	type item struct {
		idx  uint
		item T
	}
	chWr := make(chan item) // main -> worker (query)
	chRd := make(chan item) // worker -> main (result)

	// if anything goes wrong, close chWr to avoid goroutine leaks
	closed := false
	defer func() {
		if !closed {
			close(chWr)
		}
	}()

	// (1) Write the items to the main -> worker channel in a separate thread
	// of execution.  We don't need to wait for this to be done as we can
	// tell by way of chWr being closed
	go func() {
		i := 0
		for g.i.Next() {
			chWr <- item{uint(i), g.i.Get()}

			i++
		}

		close(chWr)
		closed = true
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
					chRd <- item
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
	items := make([]item, 0, 100)
	for i := range chRd {
		items = append(items, i)
	}

	// sort by the index
	slices.SortFunc(items, func(a, b item) int {
		return cmp.Compare(a.idx, b.idx)
	})

	output := make([]T, 0, len(items))
	for _, v := range items {
		output = append(output, v.item)
	}

	i := NewSliceIterator(output)
	return &i
}

func (g *Generator[T]) parallelBatchFilterNoOrder(f FilterFunc[T]) Iterator[T] {
	size := g.maxParallelism
	if sh, ok := g.i.(SizeHint[T]); ok {
		size = sh.Size()
	}

	numParallel := min(size, g.maxParallelism)

	chWr := make(chan T) // main -> worker (query)
	chRd := make(chan T) // worker -> main (result)

	// if anything goes wrong, close chWr to avoid goroutine leaks
	closed := false
	defer func() {
		if !closed {
			close(chWr)
		}
	}()

	// (1) Write the items to the main -> worker channel in a separate thread
	// of execution.  We don't need to wait for this to be done as we can
	// tell by way of chWr being closed
	go func() {
		for g.i.Next() {
			chWr <- g.i.Get()
		}

		close(chWr)
		closed = true
	}()

	// (2) Start worker go-routines.  These read items from chWr until that
	// channel is closed by the producer go-routine (1) above.
	wg := sync.WaitGroup{}
	for i := numParallel; i > 0; i-- {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for item := range chWr {
				if f(item) {
					chRd <- item
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
	items := make([]T, 0, size)
	for i := range chRd {
		items = append(items, i)
	}

	i := NewSliceIterator(items)
	return &i
}

func (g *Generator[T]) filterStreaming(f FilterFunc[T]) Iterator[T] {
	ch := make(chan T)

	fmt.Printf("FilterStream: +%v\n", *g)

	go func() {
		for g.i.Next() {
			item := g.i.Get()
			if f(item) {
				ch <- item
			}
		}
		close(ch)
	}()

	i := NewChannelIterator(ch)
	return &i
}
