package functional

type Iterator[T any] interface {
	Next() bool
	Get() T
	Error() error
}

type SizeHint[T any] interface {
	Size() uint
}

type sliceIter[T any] struct {
	s   []T
	pos int
}

func NewSliceIterator[T any](s []T) sliceIter[T] {
	return sliceIter[T]{
		s: s,
	}
}

func (t *sliceIter[T]) Size() uint {
	return uint(len(t.s))
}

func (r *sliceIter[T]) Next() bool {
	if r.pos >= len(r.s) {
		return false
	}
	r.pos++
	return true
}

func (r *sliceIter[T]) Get() T {
	return r.s[r.pos-1]
}

func (r *sliceIter[T]) Error() error {
	return nil
}

type chanIter[T any] struct {
	ch   chan T
	item T
	err  error
}

func NewChannelIterator[T any](ch chan T) chanIter[T] {
	return chanIter[T]{
		ch: ch,
	}
}

func (i *chanIter[T]) Next() bool {
	item, ok := <-i.ch
	if ok {
		i.item = item
	}
	return ok
}

func (i *chanIter[T]) Get() T {
	return i.item
}

func (i *chanIter[T]) Error() error {
	return nil
}
