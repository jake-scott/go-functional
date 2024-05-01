// Package channel implements an interator that reads a data stream from
// the supplied channel.
package channel

import "context"

// Iterator traverses the elements of type T from a channel, until
// the channel is closed.
type Iterator[T any] struct {
	ch   chan T
	item *T
	err  error
}

// New returns an implementation of Iterator that traverses the
// provided channel until reading the channel returns an error,  or the channel
// is closed.
//
// ChannelIterator does not support the SizeHint interface
func New[T any](ch chan T) Iterator[T] {
	return Iterator[T]{
		ch: ch,
	}
}

// Next reads an item from the channel and stores the value, which can be
// retrieved using the Get() method.  Next returns true if an element was
// successfully read from the channel, or false if the channel was closed or
// if the context expired.
//
// If the context expired, Err() will return the result of the context's
// Err() function.
func (i *Iterator[T]) Next(ctx context.Context) bool {
	ret := false

	select {
	case item, ok := <-i.ch:
		if ok {
			i.item = &item
			ret = true
		}

		// if ok is false, the read failed due to empty closed channel
	case <-ctx.Done():
		i.err = ctx.Err()
	}

	return ret
}

// Get returns the value stored by the last successful Next method call,
// or the zero value of type T if Next has not been called.
//
// The context is not used by this method.
func (i *Iterator[T]) Get() T {
	// return the zero value if called before Next()
	if i.item == nil {
		var ret T
		return ret
	}

	return *i.item
}

// Error returns the context expiry reason if any from a previous call
// to Next, otherwise it returns nil.
func (i *Iterator[T]) Error() error {
	return i.err
}
