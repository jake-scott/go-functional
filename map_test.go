package functional

import (
	"context"
	"fmt"
	"testing"

	"github.com/jake-scott/go-functional/iter/channel"
	"github.com/jake-scott/go-functional/iter/slice"
	"github.com/stretchr/testify/assert"
)

func stringify(i int) string {
	return fmt.Sprintf("%04d", i)
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
						out = append(out, it.Get(ctx))
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
					out = append(out, it.Get(ctx))
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
