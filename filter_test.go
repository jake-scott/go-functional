package functional

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/jake-scott/go-functional/iter/channel"
	"github.com/jake-scott/go-functional/iter/slice"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

var hundredInts = []int{
	89, 46, 43, 83, 87, 63, 48, 91, 75, 28,
	56, 21, 6, 12, 5, 39, 61, 63, 16, 23,
	81, 26, 25, 14, 9, 36, 67, 87, 30, 7,
	38, 41, 29, 13, 49, 89, 87, 34, 45, 64,
	62, 74, 70, 79, 62, 91, 4, 1, 80, 62,
	89, 17, 29, 33, 66, 3, 1, 50, 35, 86,
	74, 97, 12, 52, 72, 6, 84, 95, 31, 12,
	39, 49, 98, 11, 54, 34, 36, 7, 5, 87,
	22, 15, 20, 34, 50, 63, 43, 85, 74, 25,
	88, 7, 18, 49, 9, 26, 89, 36, 94, 60,
}

var doubleHundredInts = []int{
	178, 92, 86, 166, 174, 126, 96, 182, 150, 56,
	112, 42, 12, 24, 10, 78, 122, 126, 32, 46,
	162, 52, 50, 28, 18, 72, 134, 174, 60, 14,
	76, 82, 58, 26, 98, 178, 174, 68, 90, 128,
	124, 148, 140, 158, 124, 182, 8, 2, 160, 124,
	178, 34, 58, 66, 132, 6, 2, 100, 70, 172,
	148, 194, 24, 104, 144, 12, 168, 190, 62, 24,
	78, 98, 196, 22, 108, 68, 72, 14, 10, 174,
	44, 30, 40, 68, 100, 126, 86, 170, 148, 50,
	176, 14, 36, 98, 18, 52, 178, 72, 188, 120,
}

var hundredIntsEven = []int{
	46, 48, 28, 56, 6, 12, 16, 26, 14, 36,
	30, 38, 34, 64, 62, 74, 70, 62, 4, 80,
	62, 66, 50, 86, 74, 12, 52, 72, 6, 84,
	12, 98, 54, 34, 36, 22, 20, 34, 50, 74,
	88, 18, 26, 36, 94, 60,
}

func isEven(i int) (bool, error) {
	return i%2 == 0, nil
}

func TestFilterProcedural(t *testing.T) {
	assert := assert.New(t)

	stage := NewSliceStage(hundredInts)
	result := Filter(stage, isEven)

	assert.IsType(&Stage[int]{}, result)

	iter := result.Iterator()
	assert.IsType(&slice.Iterator[int]{}, iter)

	out := []int{}
	ctx := context.Background()
	for iter.Next(ctx) {
		out = append(out, iter.Get())
	}

	assert.EqualValues(hundredIntsEven, out)
}

func TestFilterIntsBatch(t *testing.T) {

	tr := func(f string, v ...any) {
		t.Logf(f, v...)
	}

	tests := []struct {
		name  string
		input []int
		f     FilterFunc[int]
		want  []int
	}{
		{
			name:  "find even ints from list",
			input: hundredInts,
			f:     isEven,
			want:  hundredIntsEven,
		},
		{
			name:  "find even ints from only odds",
			input: []int{1, 3, 5, 7, 9},
			f:     isEven,
			want:  []int{},
		},
		{
			name:  "empty list",
			input: []int{},
			f:     isEven,
			want:  []int{},
		},
		{
			name:  "null list",
			input: nil,
			f:     isEven,
			want:  []int{},
		},
	}

	parallelTests := []int{0, 3}
	orderedOptions := []bool{true, false}

	for _, parallel := range parallelTests {
		for _, ordered := range orderedOptions {

			for _, tt := range tests {
				name := fmt.Sprintf("%s (parallel=%d)(ordered=%v)", tt.name, parallel, ordered)
				t.Run(name, func(t *testing.T) {
					assert := assert.New(t)

					opts := []StageOption{
						PreserveOrder(ordered),
						WithTracing(true),
						WithTraceFunc(tr),
					}
					if parallel > 0 {
						opts = append(opts, Parallelism(uint(parallel)))
					}

					stage := NewSliceStage[int](tt.input, opts...)
					result := stage.Filter(tt.f)

					assert.IsType(&Stage[int]{}, result)

					it := result.Iterator()
					assert.IsType(&slice.Iterator[int]{}, it)

					out := []int{}
					ctx := context.Background()
					for it.Next(ctx) {
						out = append(out, it.Get())
					}

					// order should be preserved in seqential mode or if requested
					if ordered || parallel <= 1 {
						assert.EqualValues(tt.want, out)
					} else {
						assert.ElementsMatch(tt.want, out)
					}
				})
			}
		}
	}

	assert.NoError(t, goleak.Find())
}

func TestFilterIntsStreaming(t *testing.T) {
	tr := func(f string, v ...any) {
		t.Logf(f, v...)
	}

	tests := []struct {
		name  string
		input []int
		f     FilterFunc[int]
		want  []int
	}{
		{
			name:  "find even ints from list",
			input: hundredInts,
			f:     isEven,
			want:  hundredIntsEven,
		},
		{
			name:  "find even ints from only odds",
			input: []int{1, 3, 5, 7, 9},
			f:     isEven,
			want:  []int{},
		},
		{
			name:  "empty list",
			input: []int{},
			f:     isEven,
			want:  []int{},
		},
		{
			name:  "null list",
			input: nil,
			f:     isEven,
			want:  []int{},
		},
	}

	parallelTests := []int{0, 3}

	for _, parallel := range parallelTests {
		for _, tt := range tests {
			name := fmt.Sprintf("%s (parallel=%d)", tt.name, parallel)
			t.Run(name, func(t *testing.T) {
				assert := assert.New(t)

				opts := []StageOption{
					WithTracing(true),
					WithTraceFunc(tr),
					ProcessingType(StreamingStage),
				}
				if parallel > 0 {
					opts = append(opts, Parallelism(uint(parallel)))
				}

				stage := NewSliceStage[int](tt.input, opts...)
				result := stage.Filter(tt.f)

				assert.IsType(&Stage[int]{}, result)

				it := result.Iterator()
				assert.IsType(&channel.Iterator[int]{}, it)

				out := []int{}
				ctx := context.Background()
				for it.Next(ctx) {
					out = append(out, it.Get())
				}

				// order should be preserved in seqential mode
				if parallel <= 1 {
					assert.EqualValues(tt.want, out)
				} else {
					assert.ElementsMatch(tt.want, out)
				}
				assert.NoError(goleak.Find())
			})
		}
	}
}

func TestStalledFilterStreaming(t *testing.T) {
	assert := assert.New(t)

	tr := func(f string, v ...any) {
		t.Logf(f, v...)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := []StageOption{
		WithTracing(true),
		WithTraceFunc(tr),
		ProcessingType(StreamingStage),
		WithContext(ctx),
	}

	stage := NewSliceStage[int](hundredInts, opts...)
	result := stage.Filter(isEven)

	ctx = context.Background()
	iter := result.Iterator()

	count := 0
	for iter.Next(ctx) {
		_ = iter.Get()
		count++
		cancel()
	}

	assert.Equal(1, count)
	assert.NoError(goleak.Find())
}

type myError struct {
	err error
}

// slightly convoluted test to verify error handling when the filter function
// fails - we test the sequential and parallel code paths, ordered and unordered,
// with the error handler asking for processing to continue and to abort
func TestFailedFilterFunc(t *testing.T) {
	testVarieties := []struct {
		stageType       StageType
		parallelism     int
		ordered         bool
		continueOnError bool
	}{
		{BatchStage, 0, false, false},
		{BatchStage, 0, true, false},
		{BatchStage, 0, false, true},
		{BatchStage, 0, true, true},
		{BatchStage, 3, false, false},
		{BatchStage, 3, true, false},
		{BatchStage, 3, false, true},
		{BatchStage, 3, true, true},
		{StreamingStage, 0, false, false},
		{StreamingStage, 0, false, true},
		{StreamingStage, 3, false, false},
		{StreamingStage, 3, false, true},
	}

	// throw an error when i==66
	badFilterFunc := func(i int) (bool, error) {
		if i == 66 {
			return false, errIs66
		}

		// otherwise include the element when it is even
		return i%2 == 0, nil
	}

	for _, tt := range testVarieties {
		name := fmt.Sprintf("type=%s parallel=%d ordered=%v continue=%v", tt.stageType, tt.parallelism, tt.ordered, tt.continueOnError)
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			noError := myError{err: nil}
			var firstError atomic.Value
			firstError.Store(noError)

			errHandler := func(ec ErrorContext, err error) bool {
				firstError.CompareAndSwap(noError, myError{err})
				return tt.continueOnError
			}

			opts := []StageOption{
				PreserveOrder(tt.ordered),
				WithErrorHandler(errHandler),
				ProcessingType(tt.stageType),
			}
			if tt.parallelism > 0 {
				opts = append(opts, Parallelism(uint(tt.parallelism)))
			}

			result := NewSliceStage(hundredInts).Filter(badFilterFunc, opts...)

			assert.IsType(&Stage[int]{}, result)

			it := result.Iterator()

			if tt.stageType == BatchStage {
				assert.IsType(&slice.Iterator[int]{}, it)
			} else {
				assert.IsType(&channel.Iterator[int]{}, it)
			}

			out := []int{}
			ctx := context.Background()
			for it.Next(ctx) {
				out = append(out, it.Get())
			}

			// the results are only valid if we continued on error, but 66 should be missing
			if tt.continueOnError {
				assert.Equal(len(hundredIntsEven)-1, len(out))
			} else {
				assert.Less(len(out), len(hundredInts))
			}

			fe := firstError.Load().(myError)
			assert.NotNil(fe.err)
			assert.ErrorIs(fe.err, errIs66)
			assert.NoError(goleak.Find())
		})
	}
}

var errIs66 error = errors.New("number 66 is bad")

// badSliceIter fails when it reaches number 66
type badSliceIter struct {
	slice.Iterator[int]
	err error
}

func (i *badSliceIter) Next(ctx context.Context) bool {
	ret := i.Iterator.Next(ctx)
	if i.Get() == 66 {
		i.err = errIs66
		return false
	}

	return ret
}

func (i *badSliceIter) Error() error {
	return i.err
}

// another convoluted test of the behaviour when an iterator fails
func TestFailedFilterIterator(t *testing.T) {
	testVarieties := []struct {
		stageType       StageType
		parallelism     int
		ordered         bool
		continueOnError bool
	}{
		{BatchStage, 0, false, false},
		{BatchStage, 0, true, false},
		{BatchStage, 0, false, true},
		{BatchStage, 0, true, true},
		{BatchStage, 3, false, false},
		{BatchStage, 3, true, false},
		{BatchStage, 3, false, true},
		{BatchStage, 3, true, true},
		{StreamingStage, 0, false, false},
		{StreamingStage, 0, false, true},
		{StreamingStage, 3, false, false},
		{StreamingStage, 3, false, true},
	}

	for _, tt := range testVarieties {
		name := fmt.Sprintf("type=%s parallel=%d ordered=%v continue=%v", tt.stageType, tt.parallelism, tt.ordered, tt.continueOnError)
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			noError := myError{err: nil}
			var firstError atomic.Value
			firstError.Store(noError)

			errHandler := func(ec ErrorContext, err error) bool {
				firstError.CompareAndSwap(noError, myError{err})
				return tt.continueOnError
			}

			opts := []StageOption{
				PreserveOrder(tt.ordered),
				WithErrorHandler(errHandler),
				ProcessingType(tt.stageType),
			}
			if tt.parallelism > 0 {
				opts = append(opts, Parallelism(uint(tt.parallelism)))
			}

			iter := slice.New(hundredInts)
			myIter := badSliceIter{iter, nil}
			result := NewStage(&myIter).Filter(isEven, opts...)

			assert.IsType(&Stage[int]{}, result)

			it := result.Iterator()

			if tt.stageType == BatchStage {
				assert.IsType(&slice.Iterator[int]{}, it)
			} else {
				assert.IsType(&channel.Iterator[int]{}, it)
			}

			out := []int{}
			ctx := context.Background()
			for it.Next(ctx) {
				out = append(out, it.Get())
			}

			assert.NotNil(myIter.Error())
			assert.ErrorIs(myIter.Error(), errIs66)

			fe := firstError.Load().(myError)
			assert.NotNil(fe.err)
			assert.ErrorIs(fe.err, errIs66)

			assert.Less(len(out), len(hundredInts))
			assert.NoError(goleak.Find())
		})
	}

}

func TestFilterCancelled(t *testing.T) {
	testVarieties := []struct {
		stageType   StageType
		parallelism int
		ordered     bool
	}{
		{BatchStage, 0, false},
		{BatchStage, 0, true},
		{BatchStage, 3, false},
		{BatchStage, 3, true},
		{StreamingStage, 0, false},
		{StreamingStage, 3, false},
	}
	for _, tt := range testVarieties {
		name := fmt.Sprintf("type=%s parallel=%d ordered=%v", tt.stageType, tt.parallelism, tt.ordered)
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			noError := myError{err: nil}
			var firstError atomic.Value
			firstError.Store(noError)

			errHandler := func(ec ErrorContext, err error) bool {
				firstError.CompareAndSwap(noError, myError{err})
				return false
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// cancel the context when i==66
			cancelFilterFunc := func(i int) (bool, error) {
				if i == 66 {
					cancel()
				}

				// otherwise include the element when it is even
				return i%2 == 0, nil
			}

			opts := []StageOption{
				PreserveOrder(tt.ordered),
				WithErrorHandler(errHandler),
				WithContext(ctx),
				ProcessingType(tt.stageType),
			}
			if tt.parallelism > 0 {
				opts = append(opts, Parallelism(uint(tt.parallelism)))
			}

			result := NewSliceStage(hundredInts).Filter(cancelFilterFunc, opts...)

			assert.IsType(&Stage[int]{}, result)

			it := result.Iterator()

			if tt.stageType == BatchStage {
				assert.IsType(&slice.Iterator[int]{}, it)
			} else {
				assert.IsType(&channel.Iterator[int]{}, it)
			}

			out := []int{}
			ctx2 := context.Background()
			for it.Next(ctx2) {
				out = append(out, it.Get())
			}

			assert.Less(len(out), len(hundredInts))

			assert.NotNil(ctx.Err())
			assert.ErrorIs(ctx.Err(), context.Canceled)

			fe := firstError.Load().(myError)
			assert.NotNil(fe.err)
			assert.ErrorIs(fe.err, context.Canceled)
			assert.NoError(goleak.Find())
		})
	}
}
