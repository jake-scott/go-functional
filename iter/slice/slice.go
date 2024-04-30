// Package slice implements an iterator that traverses uni-directionally
// over a generic slice of elements
//
// Slice supports the SizeHint interface.
package slice

import "context"

// Iterator traverses over a slice of element of type T.
type Iterator[T any] struct {
	s   []T
	pos int
	err error
}

// New returns an implementation of Iterator that traverses
// over the provided slice.  The iterator returned supports the
// SizeHint itnerface.
func New[T any](s []T) Iterator[T] {
	return Iterator[T]{
		s: s,
	}
}

// Size returns the length of the underlying slice, implementing the
// SizeHint interface.
func (t *Iterator[T]) Size() uint {
	return uint(len(t.s))
}

// Next advances the iterator to the next element of the underlying
// slice.  It returns false when the end of the slice has been reached.
//
// The context is not used in this iterator implementation.
func (r *Iterator[T]) Next(ctx context.Context) bool {
	if r.pos >= len(r.s) {
		return false
	}

	if _, ok := <-ctx.Done(); !ok {
		r.err = ctx.Err()
		return false
	}

	r.pos++
	return true
}

// Get returns element of the underlying slice that the iterator refers to
//
// The context is not used in this iterator implementation.
func (r *Iterator[T]) Get(ctx context.Context) T {
	if r.pos == 0 {
		var ret T
		return ret
	}

	return r.s[r.pos-1]
}

// Error always retruns nil
func (r *Iterator[T]) Error() error {
	return r.err
}
