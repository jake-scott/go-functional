package functional

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/jake-scott/go-functional/iter/channel"
	"github.com/jake-scott/go-functional/iter/slice"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

func stringify(i int) (string, error) {
	return fmt.Sprintf("%04d", i), nil
}

func TestMapProcedural(t *testing.T) {
	assert := assert.New(t)

	stage := NewSliceStage([]int{1, 3030, 55, 787, 97})
	result := Map(stage, stringify)

	assert.IsType(&Stage[string]{}, result)

	iter := result.Iterator()
	assert.IsType(&slice.Iterator[string]{}, iter)

	out := []string{}
	ctx := context.Background()
	for iter.Next(ctx) {
		out = append(out, iter.Get())
	}

	want := []string{"0001", "3030", "0055", "0787", "0097"}

	assert.EqualValues(want, out)
}

func TestMapIntsBatch(t *testing.T) {
	tr := func(f string, v ...any) {
		t.Logf(f, v...)
	}

	tests := []struct {
		name  string
		input []int
		f     MapFunc[int, string]
		want  []string
	}{
		{
			name:  "stringify some ints",
			input: []int{1, 3030, 55, 787, 97},
			f:     stringify,
			want:  []string{"0001", "3030", "0055", "0787", "0097"},
		},
		{
			name:  "empty list",
			input: []int{},
			f:     stringify,
			want:  []string{},
		},
		{
			name:  "null list",
			input: nil,
			f:     stringify,
			want:  []string{},
		},
	}

	orderedOptions := []bool{true, false}
	parallelTests := []int{0, 3}

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
					result := Map(stage, tt.f)
					assert.IsType(&Stage[string]{}, result)

					it := result.Iterator()
					assert.IsType(&slice.Iterator[string]{}, it)

					out := []string{}
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
}

func TestMapIntsStreaming(t *testing.T) {

	tr := func(f string, v ...any) {
		t.Logf(f, v...)
	}

	tests := []struct {
		name  string
		input []int
		f     MapFunc[int, string]
		want  []string
	}{
		{
			name:  "stringify some ints",
			input: []int{1, 3030, 55, 787, 97},
			f:     stringify,
			want:  []string{"0001", "3030", "0055", "0787", "0097"},
		},
		{
			name:  "empty list",
			input: []int{},
			f:     stringify,
			want:  []string{},
		},
		{
			name:  "null list",
			input: nil,
			f:     stringify,
			want:  []string{},
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
				result := Map(stage, tt.f)
				assert.IsType(&Stage[string]{}, result)

				it := result.Iterator()
				assert.IsType(&channel.Iterator[string]{}, it)

				out := []string{}
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
			})

		}
	}
}

func TestStalledMapStreaming(t *testing.T) {
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

	stage := NewSliceStage([]int{1, 3030, 55, 787, 97}, opts...)
	result := Map(stage, stringify)

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

func TestFailedMapFunc(t *testing.T) {
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
	badMapFunc := func(i int) (int, error) {
		if i == 66 {
			return 0, errIs66
		}

		return i * 2, nil
	}

	for _, tt := range testVarieties {
		name := fmt.Sprintf("parallel=%d ordered=%v continue=%v", tt.parallelism, tt.ordered, tt.continueOnError)
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

			result := NewSliceStage(hundredInts).Map(badMapFunc, opts...)

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
				assert.Equal(len(doubleHundredInts)-1, len(out))
			} else {
				assert.Less(len(out), len(hundredInts))
			}

			fe := firstError.Load().(myError)
			assert.NotNil(fe.err)
			assert.ErrorIs(fe.err, errIs66)
		})
	}
}

func double(i int) (int, error) {
	return i * 2, nil
}

func TestFailedMapIterator(t *testing.T) {
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
			result := NewStage(&myIter).Map(double, opts...)

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

		})
	}
}

func TestMapCancelled(t *testing.T) {
	testVarieties := []struct {
		stageType   StageType
		parallelism int
		ordered     bool
	}{
		// {BatchStage, 0, false},
		// {BatchStage, 0, true},
		// {BatchStage, 3, false},
		// {BatchStage, 3, true},
		//{StreamingStage, 0, false},
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

			mapCancel := func(i int) (int, error) {
				if i == 66 {
					cancel()
				}

				return i * 2, nil
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

			result := NewSliceStage(hundredInts).Map(mapCancel, opts...)

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
		})
	}
}
