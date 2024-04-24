package functional

import (
	"context"
)

// Iterator is a generic interface for one-directional traversal through
// a collection or stream of items.
type Iterator[T any] interface {
	// Next traverses the iterator to the next element
	// Returns true if the iterator advanced, or false if there are no more
	// elements or if an error occured (see Error() below)
	Next(ctx context.Context) bool

	// Get returns current value referred to by the iterator
	Get(ctx context.Context) T

	// Error returns a non-nil value if an error occured processing Next()
	Error() error
}

// Size is an interface that can be implemented by an iterator that
// knows the number of elements in the collection when it is initialized
type Size[T any] interface {
	Size() uint
}
