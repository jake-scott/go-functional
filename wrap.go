package functional

/*  The helpers here are used in the parallel batch processors in order to
 *  optimize the unordered processing case (streaming processors are always
 *  unordered).
 *
 *  In the ordered case, every input element needs to be tagged
 *  with its index so that the results can be sorted after the processing
 *  goroutines are done.  We do that by wrapping the elements in an
 *  item[T] struct.  The wrapped element is passed to the processing
 *  goroutines which retrieve the original element from the struct.
 *
 *  In the unordered case we would like to avoid the overhead of wrapping
 *  and unwrapping the elements.  To do this, we define wrap and unwrap
 *  function templates (wrapItemFunc, unwrapItemFunc) that the processing
 *  functions call.  In the unordered case, the concrete functions that
 *  implement these templates are no-ops (they just return the origin element),
 *  and in the ordered case the functions wrap the inpuut element in an
 *  item struct.
 *
 *  The switcher functions are similar except that they return a different
 *  type than the input;  these are used in map functions.

 */

// item wraps a variable along with an index
// this is passed between the producer and consumer go-routines of a parallel
// operation so that the order can be preserved
type item[T any] struct {
	idx  uint
	item T
}

// in the function templates below:
//   T  represents the original unwrapped type (eg. string)
//   TW represents the original wrapped type (eg. item[string])
//   S  represents the new type being mapped from T (eg. int)
//   SW represente the wrapped mapped type (eg. item[int])

// a function that returns a value derived from an input value and an index
type wrapItemFunc[T any, TW any] func(i uint, t T) TW

// a function that returns the original value from a derived value
type unwrapItemFunc[TW any, T any] func(u TW) T

// a function that switches original T value from a wrapped item and replaces
// it with a new type in an output wrapped item
type switcherFunc[TW, S, SW any] func(w TW, x S) SW

// the unordered item maker just returns the input value, ignoring the
// index (it does nothing);  TW is derived to be T by the compiler
func unorderedWrapper[T any](i uint, t T) T {
	return t
}

// the unordered item getter returns the input value (does nothing)
// output element type T is derived to be TW by the compiler
func unorderedUnwrapper[TW any](tw TW) TW {
	return tw
}

// the unordered item switcher just retruns the value of the new type (x)
// SW is derived to be S by the compiler
func unorderedSwitcher[TW any, S any](tw TW, s S) S {
	return s
}

// the ordered item maker returns a wrapped item struct that includes the
// input value and index.  TW is derived to be item[T] by the compiler
func orderedWrapper[T any](i uint, t T) item[T] {
	return item[T]{
		idx:  i,
		item: t,
	}
}

// the ordered item getter returns the original value from an wrapped item
// TW is derived to be item[T] by the compiler
func orderedUnwrapper[T any](wt item[T]) T {
	return wt.item
}

// the ordered type switcher returns an item wrapper containing the
// value s of mapped type S, with the same index as the original wrapped
// item tw.
func orderedSwitcher[TW any, S any](tw item[TW], s S) item[S] {
	return item[S]{
		idx:  tw.idx,
		item: s,
	}
}
