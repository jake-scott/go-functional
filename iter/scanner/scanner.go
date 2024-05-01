// Package scanner implements a stream tokenizer iterator.
//
// The package makes use of the standard library bufio.Scanner to buffer and
// split data read from an io.Reader.  Scanner has a set of standard splitters
// for words, lines and runes and supports custom split functions as well.
package scanner

import (
	"context"
	"fmt"
)

// Iterator wraps a bufio.Scanner to traverse over a stream of tokens
// such as words or lines read from an io.Reader.
//
// Iterator does not support the SizeHint interface.
type Iterator struct {
	scanner Scanner
	err     error
}

// Scanner() is an interface defining a subset of the methods exposed by
// bufio.Scanner, and is here primarily to assist with unit testing.
type Scanner interface {
	Scan() bool
	Text() string
	Err() error
}

// ErrTooManyTokens is returned in response to a panic in the
// scanner.Scan() method, the result of too many tokens being returned without
// the scanner advancing.
type ErrTooManyTokens struct {
	panicMessage string
	err          error
}

func (e ErrTooManyTokens) Error() string {
	if e.err == nil {
		return "too many tokens: " + e.panicMessage
	} else {
		return fmt.Sprintf("too many tokens: %s", e.err)
	}
}

func (e ErrTooManyTokens) Unwrap() error {
	return e.err
}

// New returns an implementation of Iterator that uses bufio.Scanner
// to traverse through  tokens such as words or lines from an io.Reader
// such as a file.
func New(scanner Scanner) Iterator {
	return Iterator{
		scanner: scanner,
	}
}

// Next advances the iterator to the next element (scanner token) by
// calling Scanner.Scan().  It returns false if the end of the input is reached
// or an error is encountered including cancellation of the context.
// If the scanner panics, Next returns false and Error() will return the
// message from the scanner.
func (i *Iterator) Next(ctx context.Context) (ret bool) {
	defer func() {
		switch err := recover().(type) {
		default:
			i.err = ErrTooManyTokens{panicMessage: fmt.Sprintf("%v", err)}
			ret = false
		case error:
			i.err = ErrTooManyTokens{err: err}
			ret = false
		case nil:
		}
	}()

	select {
	case <-ctx.Done():
		i.err = ctx.Err()
		return false
	default:
	}

	return i.scanner.Scan()
}

// Get returns the most recent token returned by the scanner during a call
// to Next(), as a string.
//
// The context is not used in this iterator implementation.
func (i *Iterator) Get() string {
	return i.scanner.Text()
}

// Error returns the panic message from the scanner if one occured during
// a Next() call, or the cancellation message if the context was cancelled.
// Otherwise, Error calls the Scanner's Err() method, which
// returns nil if there are no errors or if the end of input is reached,
// otherwise the first error encounterd by the scanner.
func (i *Iterator) Error() error {
	if i.err != nil {
		return i.err
	}

	return i.scanner.Err()
}
