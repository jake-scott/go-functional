package functional

import (
	"context"
	"fmt"
	"testing"

	"github.com/jake-scott/go-functional/iter/slice"
	"github.com/stretchr/testify/assert"
)

const sumHundredInts = 4682

func TestReduceOO(t *testing.T) {
	assert := assert.New(t)

	add := func(a, i int) (int, error) {
		return a + i, nil
	}

	// test starting at zero
	result := NewSliceStage(hundredInts).Reduce(0, add)
	assert.Equal(sumHundredInts, result)

	// test starting at a non-zero value
	result2 := NewSliceStage(hundredInts).Reduce(123, add)
	assert.Equal(sumHundredInts+123, result2)
}

func TestReduceProc(t *testing.T) {
	assert := assert.New(t)

	// example of a reduce func that returns a different type than the
	// stage item type
	add := func(a float32, i int) (float32, error) {
		return a + float32(i), nil
	}

	// test starting at zero
	stage := NewSliceStage(hundredInts)
	result := Reduce(stage, 0, add)
	assert.IsType(float32(0), result)
	assert.Equal(float32(sumHundredInts), result)

	// test starting at a non-zero value
	stage2 := NewSliceStage(hundredInts)
	result2 := Reduce(stage2, 123, add)
	assert.IsType(float32(0), result2)
	assert.Equal(float32(sumHundredInts+123), result2)
}

func TestFailingReduceFunc(t *testing.T) {
	assert := assert.New(t)

	// Reduce func that doesn't like the number 66
	badReduceFunc := func(a int, i int) (int, error) {
		if i == 66 {
			return a, errIs66
		}
		return a + i, nil
	}

	for _, cont := range []bool{true, false} {
		name := fmt.Sprintf("cont=%v", cont)
		t.Run(name, func(t *testing.T) {

			var reduceErr error
			errHandler := func(ec ErrorContext, err error) bool {
				reduceErr = err
				return cont
			}

			// test starting at zero
			result := NewSliceStage(hundredInts).Reduce(0, badReduceFunc, WithErrorHandler(errHandler))

			assert.IsType(int(0), result)

			if cont {
				assert.Equal(sumHundredInts-66, result)
			}

			assert.NotNil(reduceErr)
			assert.ErrorIs(reduceErr, errIs66)
		})

	}
}

var sumIntsBefore66 = 2579

func TestFailingReduceIterator(t *testing.T) {
	assert := assert.New(t)

	rf := func(a int, i int) (int, error) {
		return a + i, nil
	}

	for _, cont := range []bool{true, false} {
		name := fmt.Sprintf("cont=%v", cont)
		t.Run(name, func(t *testing.T) {

			var reduceErr error
			errHandler := func(ec ErrorContext, err error) bool {
				reduceErr = err
				return cont
			}

			iter := slice.New(hundredInts)
			myIter := badSliceIter{iter, nil}

			// test starting at zero
			result := NewStage(&myIter).Reduce(0, rf, WithErrorHandler(errHandler))

			assert.IsType(int(0), result)

			if cont {
				assert.Equal(sumIntsBefore66, result)
			}

			assert.NotNil(reduceErr)
			assert.ErrorIs(reduceErr, errIs66)
		})
	}
}

func TestReduceCancelled(t *testing.T) {
	assert := assert.New(t)

	ctx, cancel := context.WithCancel(context.Background())

	var reduceErr error
	errHandler := func(ec ErrorContext, err error) bool {
		reduceErr = err
		return false
	}

	rf := func(a int, i int) (int, error) {
		if i == 66 {
			cancel()
		}
		return a + i, nil
	}

	opts := []StageOption{
		WithErrorHandler(errHandler),
		WithContext(ctx),
	}

	// test starting at zero
	result := NewSliceStage(hundredInts).Reduce(0, rf, opts...)

	assert.IsType(int(0), result)

	assert.Less(result, sumHundredInts)

	assert.NotNil(reduceErr)
	assert.ErrorIs(reduceErr, context.Canceled)
}

func TestReduceSliceFromIterator(t *testing.T) {
	assert := assert.New(t)

	s := []int{}
	result := NewSliceStage(hundredInts)
	s = Reduce(result, s, SliceFromIterator)

	assert.Equal(hundredInts, s)
}
