# go-functional: high performance functional primitives for Go

go-functional is a library that provides functional constructs for Go programs.

It can be used to construct highly parallel data processing pipelines without
having to accout for concurrency or details of passing data between processing
stages.

![GitHub tag (latest SemVer)](https://img.shields.io/github/v/tag/jake-scott/go-functional)
[![Git Workflow](https://img.shields.io/github/workflow/status/jake-scott/go-functional/unit-tests)](https://img.shields.io/github/workflow/status/jake-scott/go-functional/unit-tests)
[![Go Version](https://img.shields.io/badge/go%20version-%3E=1.21-61CFDD.svg?style=flat-square)](https://golang.org/)
[![GO Reference](https://pkg.go.dev/badge/mod/github.com/jake-scott/go-functional)](https://pkg.go.dev/mod/github.com/jake-scott/go-functional)

# Overview

go-functional provides functional constructs to Go programs.  It can be used
to create processing pipelines made up of multiple stages, each of which is
individually configurable.  The library provides:
* Filter, Map and Reduce functions for each processing stage
* A procedural and an OO interface
  * the OO interface is not supported for Map stages that produce items of
    a different type to those consumed
* Batch and stream processing modes for each stage
* Sequential and parallel processing for each stage

# Concepts

Pipelines are constructed by stiching multiple processing stages together.
The output of each stage is another stage, who's input is tied to the previous
stage's output.

Each stage reads its input using an Interator.  Iterator is an interface,
allowing the way that the initial stage consumes items, to be customized by
the developer.  go-functional supplies Iterator implementations that read
from:
* simple slices
* channels
* bufio.Scanner and like implemtations of the iter.scanner.Scanner interface

The way that stages process items can be effected by stage-wide configuration
that can also be specified at the filter/map/reduce call-sites.  Configuration
can be passed between stages by marking it as inheritable.  The following
processing parameters can be effected by configuration:
* Processing type (batch or streaming)
* Maximal degree of parallelization
* Order preservation (for batch stages)

Additionally stages can be provided with a size hint which can help reduce
allocaton overhead in batch stages that are reading an input of unknown size,
and a context to cancel background go-routines created by streaming stages.

## Processing functions

All of the processing functions receive a function object _f_ which is executed
for each input element _e_.  The processing functions produce outputs based on
the result of the function object execution _f(e)_.

### Filter

The Filter processor outputs the subset of input elements where `f(e) == true`.

### Map

The Map processor outputs one element for every input element.  The output
elements have the value of `f(e)` and may be of a different type than the input
elements.

## Reduce
<TO DO!>


# Basic usage
## Initializing the pipeline

The pipeline must be primed with an initial source of elements.  go-functional
provides three generic helper functions to create the first stage:
* `NewSliceStage[T]()` creates an initial pipeline stage that iterates over
  the supplied slice of elements of type `T`.
* `NewChannelStage[T]()` creates an initial pipeline stage that reads elements
  of type `T` from the supplied channel.
* `NewScannerStage[T]()` creates an initial pipeline stage that reads elements
  of type `T` from a bufio.Scanner implementation.  Scanner stages can be
  used to read elements from an io.Reader.

For example:
```go
type person struct {
    name string
    age  int
}
people := []person{
    {"Bob", 39},
    {"Alice", 25},
    {"Dave", 89},
    {"Mo", 20},
}
```

## Processing items

The processing functions can be used in an OO or procedural manner.  The OO
interface allows for chaining of multiple processing stages, for example:

```go
over25 := func(p person) bool {
    return p.age > 25
}

personCapitalize := func(p person) person {
    p.name = strings.ToUpper(p.name)
    return p
}

results := functional.NewSliceStage(people).
    Filter(over25).
    Map(personCapitalize)
```

The OO interface cannot be used when a map function returns elements of a
new type.  In that case the procedural interface must be used at least for
that stage:

```go
getName := func(p person) string {
    return p.name
}

results1 := functional.NewSliceStage(people).
    Filter(over25)

results := functional.Map(results1, getName)

```

## Retrieving results

The caller can extract the Iterator from the last pipeline stage and use that
to iterate over the results:

```go
iter := results.Iterator()
ctx := context.Background()
for iter.Next(ctx) {
    fmt.Printf("P: %+v\n", iter.Get(ctx))
}

// output:
// P: Bob
// P: Dave

```

# Advanced usage

## Batch vs streaming

By default, pipeline stages process their input in batch.  This means that
each stage must complete before the next proceeds.  Each stage stores the
results in a new slice that is passed to the next stage when the original
stage finishes.  This works well when the number of elements is low enough to
process in memory, and when the processing functions are fast.

When processing functions are slow (eg. use a lot of CPU or depend on
resources like the network), or when the number of items is very large, stages
can be configured to stream results to the next stage over a channel, as
results are produced.

For example:
```go

// DNS lookups are slow...
func toHostname(addr netip.Addr) string {
	hn := addr.String()

	names, err := net.LookupAddr(hn)
	if err == nil {
		hn = names[0]
	}

	return hn
}

ch := make(chan netip.Addr)

// generateIps writes IP addresses to resolve to ch
go generateIps(ch)

stage := functional.NewChannelStage(ch)
results := functional.Map(stage, toHostname, functional.ProcessingType(functional.StreamingStage))

// read results as they are produced by Map()
iter := results.Iterator()
for iter.Next(ctx) {
    fmt.Println(iter.Get(ctx))
}

```

## Sequential vs parallel processing

By default, pipeline stages process input elements one at a time, whether
processing in batch or streaming.  This works when the processing function
can keep up with the pace of input elements (if streaming) or is otherwise
fast.

When the processing function is too slow, the whole pipeline slows down.  The
slow stage can be run in parallel to alleviate the stall.

For example:
```go
results := functional.Map(stage, toHostname,
    functional.ProcessingType(functional.StreamingStage),
    functional.Parallelism(10))
```

Note that the requested degree of parallelism is used in conjunction  with the
size hint for a stage.  When an underlying iterator cannot supply its own size
hint (eg. channels or scanners), a stage uses a user supplied hint or otherwise
defaults to 100.

The maximum degree of parallelism actually used by a stage is the minimum
of the size hint and the value of the Parallelism() option.  So it
may be necessary to supply the _SizeHint_ option as well as the _Parallelism_
option especially when the desired degree of parallelism is over 100.

Of course larger degrees of parallelism are suitable for non-CPU bound stages
like network access where as smaller degrees of parallelism are suited more
for CPU bound stages.

## Inherited options

Whenever options are passed to an initial stage or a processing function, the
`InheritOptions` option may also be supplied to cause all the supplied options
to be inherited by future stages.  These options can the be overridden per
processing function, eg:
```
results := functional.NewSliceStage(people,
	    functional.InheritOptions(true),
		functional.WithTracing(true)).
    Filter(over25).
    Map(personCapitalize)
```

Non-inherited options passed to the initial stage constructor do not have any
effect.

## Contexts

A context can be passed to any stage.  The context is used to abort processing
when the context is cancelled or expires.  The iterator `Next()` and `Get()`
methods also accept a context to allow interruption of blocking reads
(eg. on channels or scanners).  The processing functions pass the stage context
to the iterator methods, and the caller must also pass a context to these
methods when extracting the results from the pipeline.

A context **should** be passed to a pipeline or stage whenever there are parallel
or streaming stages, to avoid goroutine leaks.

