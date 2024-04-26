package functional

import (
	"bufio"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jake-scott/go-functional/iter/channel"
	"github.com/jake-scott/go-functional/iter/scanner"
	"github.com/jake-scott/go-functional/iter/slice"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

func mkStageOpts1() []StageOption {
	opts := []StageOption{
		ProcessingType(StreamingStage),
		Parallelism(10),
		SizeHint(20),
		PreserveOrder(true),
		WithContext(context.Background()),
		InheritOptions(true),
	}

	return opts
}

func TestNewStage(t *testing.T) {
	opts := mkStageOpts1()
	assert := assert.New(t)

	i := slice.New([]string{})

	s := NewStage(&i)
	assert.Equal(BatchStage, s.opts.stageType)
	assert.Equal(uint(0), s.opts.maxParallelism)
	assert.Equal(DefaultSizeHint, s.opts.sizeHint)
	assert.Equal(false, s.opts.preserveOrder)
	assert.Equal(false, s.opts.inheritOptions)
	assert.NotNil(s.opts.ctx)

	s = NewStage(&i, opts...)
	assert.Equal(StreamingStage, s.opts.stageType)
	assert.Equal(uint(10), s.opts.maxParallelism)
	assert.Equal(uint(20), s.opts.sizeHint)
	assert.Equal(true, s.opts.preserveOrder)
	assert.Equal(true, s.opts.inheritOptions)
	assert.NotNil(s.opts.ctx)
}

func TestNewSliceStage(t *testing.T) {
	opts := mkStageOpts1()
	assert := assert.New(t)

	vals := []string{}

	s := NewSliceStage(vals)
	assert.Equal(BatchStage, s.opts.stageType)
	assert.Equal(uint(0), s.opts.maxParallelism)
	assert.Equal(DefaultSizeHint, s.opts.sizeHint)
	assert.Equal(false, s.opts.preserveOrder)
	assert.Equal(false, s.opts.inheritOptions)
	assert.NotNil(s.opts.ctx)
	assert.IsType(&slice.Iterator[string]{}, s.i)
	assert.IsType(&slice.Iterator[string]{}, s.Iterator())

	s = NewSliceStage(vals, opts...)
	assert.Equal(StreamingStage, s.opts.stageType)
	assert.Equal(uint(10), s.opts.maxParallelism)
	assert.Equal(uint(20), s.opts.sizeHint)
	assert.Equal(true, s.opts.preserveOrder)
	assert.Equal(true, s.opts.inheritOptions)
	assert.NotNil(s.opts.ctx)
	assert.IsType(&slice.Iterator[string]{}, s.i)
	assert.IsType(&slice.Iterator[string]{}, s.Iterator())
}

func TestNewChannelStage(t *testing.T) {
	opts := mkStageOpts1()
	assert := assert.New(t)

	ch := make(chan string)

	s := NewChannelStage(ch)
	assert.Equal(BatchStage, s.opts.stageType)
	assert.Equal(uint(0), s.opts.maxParallelism)
	assert.Equal(DefaultSizeHint, s.opts.sizeHint)
	assert.Equal(false, s.opts.preserveOrder)
	assert.Equal(false, s.opts.inheritOptions)
	assert.NotNil(s.opts.ctx)
	assert.IsType(&channel.Iterator[string]{}, s.i)
	assert.IsType(&channel.Iterator[string]{}, s.Iterator())

	s = NewChannelStage(ch, opts...)
	assert.Equal(StreamingStage, s.opts.stageType)
	assert.Equal(uint(10), s.opts.maxParallelism)
	assert.Equal(uint(20), s.opts.sizeHint)
	assert.Equal(true, s.opts.preserveOrder)
	assert.Equal(true, s.opts.inheritOptions)
	assert.NotNil(s.opts.ctx)
	assert.IsType(&channel.Iterator[string]{}, s.i)
	assert.IsType(&channel.Iterator[string]{}, s.Iterator())
}

func TestNewScannerStage(t *testing.T) {
	opts := mkStageOpts1()
	assert := assert.New(t)

	sc := bufio.NewScanner(strings.NewReader("test"))

	s := NewScannerStage(sc)
	assert.Equal(BatchStage, s.opts.stageType)
	assert.Equal(uint(0), s.opts.maxParallelism)
	assert.Equal(DefaultSizeHint, s.opts.sizeHint)
	assert.Equal(false, s.opts.preserveOrder)
	assert.Equal(false, s.opts.inheritOptions)
	assert.NotNil(s.opts.ctx)
	assert.IsType(&scanner.Iterator{}, s.i)
	assert.IsType(&scanner.Iterator{}, s.Iterator())

	s = NewScannerStage(sc, opts...)
	assert.Equal(StreamingStage, s.opts.stageType)
	assert.Equal(uint(10), s.opts.maxParallelism)
	assert.Equal(uint(20), s.opts.sizeHint)
	assert.Equal(true, s.opts.preserveOrder)
	assert.Equal(true, s.opts.inheritOptions)
	assert.NotNil(s.opts.ctx)
	assert.IsType(&scanner.Iterator{}, s.i)
	assert.IsType(&scanner.Iterator{}, s.Iterator())
}

func TestParallelProcessor(t *testing.T) {
	assert := assert.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	iterIn := slice.New(hundredInts)
	tr := NewTracer(1, "test", func(f string, v ...any) {
		t.Logf(f, v...)
	})

	ch := parallelProcessor[int, int, int](ctx, 5, &iterIn, tr,
		func(idx uint, i int, ch chan int) {
			ch <- i
		},
		func(i int, ch chan int) error {
			ch <- i
			return nil
		})

	assert.IsType(make(chan int), ch)

	results := make([]int, 0, len(hundredInts))
	for v := range ch {
		results = append(results, v)
	}

	// results will not be in order..
	assert.ElementsMatch(hundredInts, results)
	assert.NoError(goleak.Find())
}

func TestParallelProcessorCancelled(t *testing.T) {
	assert := assert.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	iterIn := slice.New(hundredInts)
	tr := NewTracer(1, "test", func(f string, v ...any) {
		t.Logf(f, v...)
	})

	ch := parallelProcessor[int, int, int](ctx, 5, &iterIn, tr,
		func(idx uint, i int, ch chan int) {
			ch <- i
		},
		func(i int, ch chan int) error {
			select {
			case ch <- i:
			case <-ctx.Done():
				t.Logf("Cancelled")
				return ctx.Err()
			}
			return nil
		})

	assert.IsType(make(chan int), ch)

	cancel()
	time.Sleep(2 * time.Second)
	assert.NoError(goleak.Find())
}
