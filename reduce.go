package functional

// ReduceFunc is a generic function that takes an element value of type T
// and an accululator value of type A and returns a new accumulator value.
//
// Example:
//
//	func add(a, i int) int {
//	    return a + i
//	}
type ReduceFunc[T any, A any] func(A, T) A

// Reduce processes the stage's input elements to a single element of the same
// type, by calling r for every element and passing an accumulator value
// that each invocation of r can update by returning a value.
//
// If the Reduce function returns a value of a different type to the input
// values, the non-OO version of Reduce() must be used instead.
//
// Reduce always runs sequentially in a batch mode.
func (s *Stage[T]) Reduce(initial T, r ReduceFunc[T, T], opts ...StageOption) T {
	return Reduce(s, initial, r, opts...)
}

// Reduce is the non-OO version of stage.Reduce().  It must be used in the case
// where the accumulator of the reduce function is of a different type to
// the input elements (due to limitations of go generics).
func Reduce[T, A any](s *Stage[T], initial A, r ReduceFunc[T, A], opts ...StageOption) A {
	// opts for this Reduce are the stage options overridden by the reduce
	// specific options passed to this call
	merged := *s
	merged.opts.processOptions(opts...)

	t := merged.tracer("Reduce")
	defer t.End()

	accum := initial
	for s.i.Next(s.opts.ctx) {
		accum = r(accum, s.i.Get(s.opts.ctx))
	}

	return accum
}

// Convenience reduction function that returns a slice of elements from the
// iterator of the pipeline stage.
func SliceFromIterator[T any](a []T, t T) []T {
	return append(a, t)
}
