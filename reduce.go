package functional

// ReduceFunc is a generic function that takes an element value of type T
// and an accululator value of type A and returns a new accumulator value.
//
// Example:
//
//	func add(a, i int) (int, error) {
//	    return a + i, nil
//	}
type ReduceFunc[T any, A any] func(A, T) (A, error)

// Reduce processes the stage's input elements to a single accumulator value
// of type A, by calling r for every element and passing an accumulator value
// that each invocation of r can update by returning a value.  A may differ
// from the input type T and is normally inferred from the initial value and
// the reduce function.
//
// Reduce always runs sequentially in a batch mode.
func (s *Stage[T]) Reduce[A any](initial A, r ReduceFunc[T, A], opts ...StageOption) A {
	return Reduce(s, initial, r, opts...)
}

// Reduce is the procedural equivalent of Stage.Reduce().
func Reduce[T, A any](s *Stage[T], initial A, r ReduceFunc[T, A], opts ...StageOption) A {
	// opts for this Reduce are the stage options overridden by the reduce
	// specific options passed to this call
	merged := *s
	merged.opts.processOptions(opts...)

	t := merged.tracer("Reduce")
	defer t.end()

	accum := initial
	var err error

reduceLoop:
	for s.i.Next(merged.opts.ctx) {
		// select {
		// case <-merged.opts.ctx.Done():
		// 	break reduceLoop
		// default:

		accum, err = r(accum, s.i.Get())
		if err != nil {
			if !merged.opts.onError(ErrorContextFilterFunction, err) {
				t.msg("reduce done due to error: %s", err)
				break reduceLoop
			}
		}
		// }
	}

	if s.i.Error() != nil {
		merged.opts.onError(ErrorContextItertator, s.i.Error())
	}

	if merged.opts.ctx.Err() != nil {
		merged.opts.onError(ErrorContextOther, merged.opts.ctx.Err())
	}

	return accum
}

// Convenience reduction function that returns a slice of elements from the
// iterator of the pipeline stage.
func SliceFromIterator[T any](a []T, t T) ([]T, error) {
	return append(a, t), nil
}
