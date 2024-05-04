// Package functional provides highly performant functional primitives
// for Go.  It supports streaming and parallel execution of stages
// of a processing pipeline.
package functional

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/jake-scott/go-functional/iter/channel"
	"github.com/jake-scott/go-functional/iter/scanner"
	"github.com/jake-scott/go-functional/iter/slice"
)

// DefaultSizeHint is used by batch processing functions for initial allocations
// when the underlying iterator cannot provide size infomation and a stage
// specific size hint has not been provided.
var DefaultSizeHint uint = 100

// StageType describes the behaviour of a pipeline stage
type StageType int

const (
	// Batch stages collect the results of processing all of the
	// input items before passing control to the next stage
	BatchStage StageType = iota

	// Streaming stages pass the results of processed input items to the
	// next pipeline stage as a stream while processing other elements continues.
	StreamingStage
)

func (t StageType) String() string {
	switch t {
	default:
		return "unknown"
	case BatchStage:
		return "Batch"
	case StreamingStage:
		return "Streaming"
	}
}

// ErrorContext provides error handler callbacks with a hint about where in
// processing the error occured
type ErrorContext int

const (
	// ErrorContextIterator hints that the error occured reading an interator
	ErrorContextItertator ErrorContext = iota

	// ErrorContextFilterFunction means the error occured in a filter func
	ErrorContextFilterFunction

	// ErrorContextMapFunction means the error occured in a map func
	ErrorContextMapFunction

	// ErrorContextReduceFunction means the error occued in a reduce func
	ErrorContextReduceFunction

	// We don't know which phase of processing the error occured when
	// the hint it ErrorContextOther
	ErrorContextOther
)

// Functions complying with the ErrorHandler prototype can be used to process
// errors that occur during the pipeline processing functions.  The default
// handler ignores the error.  A custom handler can be provided using the
// WithErrorHandler option.
//
// Parameters:
//   - where describes the context in which the error occured
//   - err is the error to be handled
//
// The function should return true if processing should continue regardless,
// or false to stop processing.
type ErrorHandler func(where ErrorContext, err error) bool

func nullErrorHandler(ErrorContext, error) bool {
	return true
}

var stageCounter atomic.Uint32

// Stage represents one processing phase of a larger pipeline
// The processing methods of a stage read input elements using the underlying
// Iterator and return a new Stage ready to read elements from the previous
// stage using a new iterator.
type Stage[T any] struct {
	i    Iterator[T]
	id   uint32
	wg   *sync.WaitGroup
	opts stageOptions
}

type stageOptions struct {
	stageType      StageType
	maxParallelism uint
	preserveOrder  bool
	inheritOptions bool
	sizeHint       uint
	tracer         TraceFunc
	tracing        bool
	ctx            context.Context
	onError        ErrorHandler
}

// StageOptions provide a mechanism to customize how the processing functions
// of a stage opterate.
type StageOption func(g *stageOptions)

// The ProcessingType option configures whether the stage operates in batch
// or streaming mode.  If not specified, stages default to processing in
// batch mode.
func ProcessingType(t StageType) StageOption {
	return func(o *stageOptions) {
		o.stageType = t
	}
}

// The Parallem option defines the maximum concurrency of the stage.
//
// If not specified, the default is to process elements serially.
func Parallelism(max uint) StageOption {
	return func(o *stageOptions) {
		o.maxParallelism = max
	}
}

// The SizeHint option provides the stage processor functions with a guideline
// regarding the number of elements there are to process.  This is primarily
// used with iterators that cannot provide the information themselves.
//
// If not specified and the iterator cannot provide the information, the default
// value DefaultSizeHint is used.
func SizeHint(hint uint) StageOption {
	return func(o *stageOptions) {
		o.sizeHint = hint
	}
}

// PreserveOrder causes concurent batch stages to retain the order of
// processed elements.  This is always the case with serial stages and is
// not possible for concurrent streaming stages.  Maintaining the order of
// elements for concurrent batch stages incurs a performance penalty.
//
// The default is to not maintain order.
func PreserveOrder(preserve bool) StageOption {
	return func(o *stageOptions) {
		o.preserveOrder = preserve
	}
}

// WithContext attaches the provided context to the stage.
func WithContext(ctx context.Context) StageOption {
	return func(o *stageOptions) {
		o.ctx = ctx
	}
}

// WithTraceFunc sets the trace function for the stage.  Use WithTracing
// to enable/disable tracing.
func WithTraceFunc(f TraceFunc) StageOption {
	return func(o *stageOptions) {
		o.tracer = f
	}
}

// WithTracing enables tracing for the stage.  If a custom trace function
// has not been set using WithTraceFunc, trace messages are printed to stderr.
func WithTracing(enable bool) StageOption {
	return func(o *stageOptions) {
		o.tracing = enable
	}
}

// WithErrorHandler installs a custom error handler which will be called
// from the processing functions when the filter/map/reduce function or
// an iterator emits an error.
//
// The handler should return true to continue processing or false to abort.
//
// The handler can stash the error for use in the pipeline's caller.
func WithErrorHandler(handler ErrorHandler) StageOption {
	return func(o *stageOptions) {
		o.onError = handler
	}
}

// InheritOptions causes this stage's options to be inherited by the next
// stage.  The next stage can override these inherited options.  Further
// inheritence can be disabled by passing this option with a false value.
//
// The default is no inheritence.
func InheritOptions(inherit bool) StageOption {
	return func(o *stageOptions) {
		o.inheritOptions = inherit
	}
}

func (o *stageOptions) processOptions(opts ...StageOption) {
	for _, f := range opts {
		f(o)
	}
}

// NewStage instantiates a pipeline stage from an Iterator and optional
// set of processing optionns
func NewStage[T any](i Iterator[T], opts ...StageOption) *Stage[T] {
	s := &Stage[T]{
		i: i,
		opts: stageOptions{
			ctx:      context.Background(),
			sizeHint: DefaultSizeHint,
			onError:  nullErrorHandler,
		},
		id: stageCounter.Add(1),
		wg: &sync.WaitGroup{},
	}
	s.opts.processOptions(opts...)
	return s
}

// NewSliceStage instantiates a pipeline stage using a slice iterator backed by
// the provided slice.
func NewSliceStage[T any](s []T, opts ...StageOption) *Stage[T] {
	iter := slice.New(s)
	return NewStage(&iter, opts...)
}

// NewChannelStage instantiates a pipeline stage using a channel iterator
// backed by the provided channel.
func NewChannelStage[T any](ch chan T, opts ...StageOption) *Stage[T] {
	iter := channel.New(ch)
	return NewStage(&iter, opts...)
}

// NewScannerState instantiates a pipeline stage using a scanner iterator,
// backed by the provided scanner.
func NewScannerStage(s scanner.Scanner, opts ...StageOption) *Stage[string] {
	iter := scanner.New(s)
	return NewStage(&iter, opts...)
}

// Iterator returns the underlying iterator for a stage.  It is most useful
// as a mechanism for retrieving the result from the last stage of a pipeline
// by the caller of the pipeline.
func (s *Stage[T]) Iterator() Iterator[T] {
	return s.i
}

func (s *Stage[T]) tracer(description string, v ...any) tracer {
	if s.opts.tracing {
		var t T
		description = fmt.Sprintf("(%T) %s", t, description)
		return newTracer(s.id, description, s.opts.tracer, v...)
	} else {
		return nullTracer{}
	}
}

func (s *Stage[T]) nextStage(i Iterator[T], opts ...StageOption) *Stage[T] {
	return nextStage(s, i, opts...)
}

func nextStage[T, U any](s *Stage[T], i Iterator[U], opts ...StageOption) *Stage[U] {
	nextStage := &Stage[U]{
		i:  i,
		id: stageCounter.Add(1),
		wg: s.wg,
	}

	// if this stage has inheritence enabled them copy its options to the
	// next stage
	if s.opts.inheritOptions {
		nextStage.opts = s.opts
	} else {
		nextStage.opts = stageOptions{
			ctx:      context.Background(),
			sizeHint: DefaultSizeHint,
			onError:  nullErrorHandler,
		}
	}

	// process new options on their own to see if we should inherit
	var newOpts stageOptions
	newOpts.processOptions(opts...)

	// .. if so then merge the new opts with the stage options
	if newOpts.inheritOptions {
		nextStage.opts.processOptions(opts...)
	}

	return nextStage
}

// parallelProcessor reads values from iter in a producer go-routine, and calls push() for
// each element.  The push function should write an element to ch.
// numParallel worker goroutines read elements from from the producer goroutine and call
// pull() for each element.  The pull function should write possibly new elements to ch.
// The return value is a channel to which unordered results can be read.
//
// T:  source item type
// TW: wrapped source item type
// MW: wrapped result item type (same as TW for filters, possibly different than TW for maps)
func parallelProcessor[T, TW, MW any](opts stageOptions, numParallel uint, iter Iterator[T], t tracer,
	push func(context.Context, uint, T, chan TW), pull func(TW, chan MW) error) chan MW {

	chWorker := make(chan TW) // channel towards to workers
	chOut := make(chan MW)    // worker output channel

	ctx, cancel := context.WithCancel(opts.ctx)

	wgReader := sync.WaitGroup{}
	wgReader.Add(1)

	chStop := make(chan struct{})

	// (1) Read items from the iterator in a separate goroutine, until done or
	//     the context expires, then write the items to the worker channel
	go func() {
		t := t.subTracer("reader")
		defer wgReader.Done()
		defer t.end()

		// close chWorker when done.. this will cause the workers to terminate
		// when they have processed the items
		defer func() {
			close(chWorker)
		}()

		i := 0
	iterLoop:
		for iter.Next(ctx) {
			// run the push function which should write all items to chWorker
			select {
			case <-chStop: // if the workers terminate first, this tells us to stop
				break iterLoop
			default:
				push(ctx, uint(i), iter.Get(), chWorker)
			}
			i++
		}

		// if there is an iterator read error, report it to the error handler
		// even though there isn't anything we can do to abort the next stages
		// .. we can at least stop sending new items
		if iter.Error() != nil {
			opts.onError(ErrorContextItertator, iter.Error())
		}
	}()

	// (2) Start worker go-routines.  These read items from chWorker until that
	// channel is closed by the producer go-routine (1) above.
	wgWorker := sync.WaitGroup{}
	for i := uint(0); i < numParallel; i++ {
		wgWorker.Add(1)

		i := i
		go func() {
			t := t.subTracer("processor %d", i)

			defer wgWorker.Done()
			defer t.end()

		readLoop:
			for {
				select {
				case item, ok := <-chWorker:
					if ok {
						// run the pull function which should selectively write
						// items to chOut depending on functionality
						err := pull(item, chOut)
						if err != nil {
							if !opts.onError(ErrorContextOther, err) {
								cancel()
								continue
							}
						}
					} else {

						// if not OK the read failed on an empty, closed channel
						break readLoop
					}
				case <-ctx.Done():
					t.msg("cancelled")
					opts.onError(ErrorContextOther, ctx.Err())
					break readLoop
				}
			}
		}()
	}

	// (3) Wait for the workers in a separate go-routine and close the result
	// channel once they are all done
	go func() {
		t := t.subTracer("wait for processors")

		wgWorker.Wait()
		close(chOut)
		close(chStop)

		wgReader.Wait()

		t.end()
		cancel()
	}()

	return chOut
}

func closeChanIfOpen[T any](ch chan T) {
	ok := true
	select {
	case _, ok = <-ch:
	default:
	}
	if ok {
		defer func() {
			_ = recover()
		}()

		close(ch)
	}
}
