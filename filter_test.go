package functional

import (
	"context"
	"fmt"
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

var hundredIntsEven = []int{
	46, 48, 28, 56, 6, 12, 16, 26, 14, 36,
	30, 38, 34, 64, 62, 74, 70, 62, 4, 80,
	62, 66, 50, 86, 74, 12, 52, 72, 6, 84,
	12, 98, 54, 34, 36, 22, 20, 34, 50, 74,
	88, 18, 26, 36, 94, 60,
}

func TestCloseChanIfOpen(t *testing.T) {
	assert := assert.New(t)

	// test with an unbuffered channel:
	//  after calling closeChanIfOpen, writes and closes should panic but
	//  further closeChanIfOpen calls should not
	ch1 := make(chan string)
	closeChanIfOpen(ch1)
	assert.Panics(func() { ch1 <- "test" })
	assert.Panics(func() { close(ch1) })
	assert.NotPanics(func() { closeChanIfOpen(ch1) })

	// test with an unbuffered channel that we pre-close:
	//   writes and closes should panic bug closeChanIfOpen should not
	ch2 := make(chan string)
	close(ch2)
	assert.Panics(func() { ch2 <- "test" })
	assert.Panics(func() { close(ch2) })
	assert.NotPanics(func() { closeChanIfOpen(ch2) })

	// test with a buffered channel:
	//   after calling closeChanIfOpen, writes and closes should panic but
	//   further closeChanIfOpen calls should not
	ch3 := make(chan string, 10)
	assert.NotPanics(func() { ch3 <- "test" })
	assert.NotPanics(func() { ch3 <- "test" })
	closeChanIfOpen(ch3)
	assert.Panics(func() { ch3 <- "test" })
	assert.Panics(func() { close(ch3) })
	assert.NotPanics(func() { closeChanIfOpen(ch3) })

	//same as above but we pre-close the cahnnel
	ch4 := make(chan string, 10)
	assert.NotPanics(func() { ch4 <- "test" })
	assert.NotPanics(func() { ch4 <- "test" })
	close(ch4)
	assert.Panics(func() { ch4 <- "test" })
	assert.Panics(func() { close(ch4) })
	assert.NotPanics(func() { closeChanIfOpen(ch2) })
}

func isEven(i int) (bool, error) {
	return i%2 == 0, nil
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
			})
		}
	}

	assert.NoError(t, goleak.Find())
}

func TestStalledStreaming(t *testing.T) {
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
