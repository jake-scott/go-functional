# go-functional: High-Performance functional primitives for Go

go-functional is a library that provides functional constructs for Go programs.



![GitHub tag (latest SemVer)](https://img.shields.io/github/v/tag/jake-scott/go-functional)
[![Git Workflow](https://img.shields.io/github/workflow/status/jake-scott/go-functional/unit-tests)](https://img.shields.io/github/workflow/status/jake-scott/go-functional/unit-tests)
[![Go Version](https://img.shields.io/badge/go%20version-%3E=1.21-61CFDD.svg?style=flat-square)](https://golang.org/)
[![GO Reference](https://pkg.go.dev/badge/mod/github.com/jake-scott/go-functional)](https://pkg.go.dev/mod/github.com/jake-scott/go-functional)

# Overview

go-functional is a high-performance library that provides functional constructs
for Go programs. It allows developers to easily construct highly parallel data
processing pipelines without worrying about concurrency or the intricacies of
data transfer between stages.

## Key Features

* Filter, Map, and Reduce functions for each processing stage
* Procedural and object-oriented interfaces
* Batch and stream processing modes
* Sequential and parallel processing
* Configurable parameters for fine-grained control
* Context support for cancellation and resource management

## Installation

```bash
go get github.com/jake-scott/go-functional
```

# Usage

## Basic Example

```go
type Person struct {
    Name string
    Age  int
}

people := []Person{
    {"Alice", 30},
    {"Bob", 25},
    {"Charlie", 40},
}

// Filter adults
adults := functional.NewSliceStage(people).
    Filter(func(p Person) (bool, error) { return p.Age >= 30, nil })

// Map names to uppercase
names := functional.Map(adults, func(p Person) (string, error) { return strings.ToUpper(p.Name), nil })

// Iterate over results
ctx := context.Background()
for names.Iterator().Next(ctx) {
    fmt.Println(names.Iterator().Get()) // Output: ALICE, CHARLIE
}
```

## Advanced Features
* Batch vs. streaming processing
* Sequential vs. parallel processing
* Inherited options
* Context cancellation

# Concepts

Pipelines are constructed by stiching multiple processing stages together.
The output of each stage becomes the input for the next stage.

Each stage reads its input using an Iterator. Iterator is an interface, allowing
the developer to customize the way the initial stage consumes items. go-functional
supplies Iterator implementations that read from:
* simple slices
* channels
* bufio.Scanner and like implemtations of the iter.scanner.Scanner interface

The way stages process items can be affected by configuration supplied to the
first stage when it is constructed, or to per-stage filter/map/reduce calls.
Configuration can be passed between stages by marking it as inheritable.
The following processing parameters can be affected by configuration:

* Processing type (batch or streaming)
* Maximal degree of parallelization
* Order preservation (for batch stages)

Additionally, stages can be provided with a size hint, which can help reduce
allocation overhead in batch stages reading an input of unknown size, and a
context to cancel background goroutines created by streaming and parallel stages.


## Processing functions

All processing functions are passed a function object _f_ which is executed for
each input element _e_. The processing functions produce outputs based on the
result of the function object execution _f(e)_.


### Filter

The Filter processor outputs the subset of input elements where `f(e) == true`.

### Map

The Map processor outputs one element for every input element.  The output
elements have the value of `f(e)` and may be of a different type than the input
elements.

### Reduce

The Reduce processor outputs a single element that is the result of running
a reduce function on every element.  Reduce functions mutate an accumulator
as elements are processed, by returning the new version of the accumulator.


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

pipeline := NewSliceStage(people)
```

## Processing items

The processing functions can be used in an object-oriented or procedural manner.
The object-oriented interface allows for chaining of multiple processing stages,
for example:

```go
over25 := func(p person) (bool, error) {
    return p.age > 25, nil
}

personCapitalize := func(p person) (person, error) {
    p.name = strings.ToUpper(p.name)
    return p, nil
}

results := functional.NewSliceStage(people).
    Filter(over25).
    Map(personCapitalize)
```

The object-oriented interface cannot be used when a map function returns elements
of a new type. In that case, the procedural interface must be used at least for
that stage:

```go
getName := func(p person) (string, error) {
    return p.name, nil
}

results1 := functional.NewSliceStage(people).
    Filter(over25)

results := functional.Map(results1, getName)

```

## Retrieving results

The caller reads results from the Iterator from the last pipeline stage:

```go
iter := results.Iterator()
ctx := context.Background()
for iter.Next(ctx) {
    fmt.Printf("P: %+v\n", iter.Get())
}

// output:
// P: Bob
// P: Dave

```

# Advanced usage

## Batch vs streaming

By default, pipeline stages process their input in batch mode.  This means that
each stage must complete before the next can proceed.  Each stage stores the
results in a new slice that is passed to the next stage when the original
stage finishes.  This approach works well when the number of elements is low
enough to process in memory, and when the processing functions are fast.

When processing functions are slow (eg. use a lot of CPU or depend on
external resources like the network), or when the number of items is very large,
stages can be configured to stream results to the next stage over a channel, as
results are produced.

For example:
```go
// DNS lookups are slow...
func toHostname(addr netip.Addr) (string, error) {
    hn := addr.String()

    names, err := net.LookupAddr(hn)
    if err == nil {
        hn = names[0]
    }

    return hn, nil
}

ch := make(chan netip.Addr)

// generateIps writes IP addresses to resolve to ch
go generateIps(ch)

stage := functional.NewChannelStage(ch)
results := functional.Map(stage, toHostname, functional.ProcessingType(functional.StreamingStage))

// read results as they are produced by Map()
iter := results.Iterator()
for iter.Next(ctx) {
    fmt.Println(iter.Get())
}
```

## Sequential vs parallel processing

By default, pipeline stages process input elements one at a time, irrespective
of whether they are in batch or streaming mode.. This approach works when the
processing function can keep up with the pace of input elements (in the case
of streaming) or when the function itself if fast.

When a processing function is slow, it can become a bottleneck for the entire
pipeline. To address this, the slow stage can be run in parallel to increase
performance.

For example:
```go
results := functional.Map(stage, toHostname,
    functional.ProcessingType(functional.StreamingStage),
    functional.Parallelism(10))
```


Note that the requested degree of parallelism is used in conjunction with the
size hint for a stage. When an underlying iterator cannot supply its own size
hint (e.g., channels or scanners), a stage uses a user-supplied hint or
otherwise defaults to 100 (which can be changed by updating the value of the
`DefaultSizeHint` variable).

The maximum degree of parallelism used by a stage is the minimum of the size
hint and the value of the Parallelism() option. Hence, it may be necessary to
supply both the SizeHint and Parallelism options, especially when aiming for
a degree of parallelism exceeding 100.

Larger degrees of parallelism are suitable for non-CPU bound stages like network
access, whereas smaller degrees of parallelism are more suited for CPU-bound
stages.


## Inherited options

Whenever options are passed to an initial stage or a processing function, the
`InheritOptions` option can also be supplied to enable the inheritance of all
the provided options by future stages. These inherited options can then be
overridden per processing function. For example:

```go
join := func(a, b string) (string, error) {
    if a == "" {
        return b, nil
    } else {
        return fmt.Sprintf("%s, %s", a, b), nil
    }
}

results := functional.NewSliceStage(people,
              functional.InheritOptions(true),
              functional.WithTracing(true)).
            Filter(over25).
            Map(personCapitalize).
            Reduce("", join)

fmt.Println(results)        // Output: ALICE, CHARLIE
```

Non-inherited options passed to the initial stage constructor do not have any
effect.

## Contexts

A context can be passed to any stage using the `WithContext` option.  This 
allows the pipeline to be canceled when the context is canceled or expires.
The iterator `Next()` method also accept a context to enable the interruption of
blocking reads (e.g., on channels or scanners). The processing functions pass the stage context to the iterator methods, and the caller must also pass a context to these
methods when extracting the results from the pipeline.

It is recommended to use a context for pipelines or stages that involve parallel
or streaming stages to avoid goroutine leaks.

## Error handling

There are two sources of errors that can occur during processing:
 * The source iterator (eg. an error reading from a network)
 * User supplied filter/map/reduce functions

go-functional runs an error handler callback whenever one of these conditions is
encountered.  The error handler can indicate that the pipeline processing should continue
or be aborted by returning true (continue) or false (abort).  The default error handler
does nothing and indicates that processing should continue.

A custom error handler can be passed as an option to any stage, eg:

```go
func logErrorHandler(ec functional.ErrorContext, err error) bool {
	log.Printf("Error in %v: %s\n", ec, err)
	return false    // abort processing
}

stage := functional.NewSliceStage(people, functional.WithErrorHandler(logErrorHandler))
```

It is possible to have the error handler stash the error for retreival after the
pipeline returns control to the caller.


# API Reference

For detailed information on functions and types, please refer to the
[API documentation](https://pkg.go.dev/mod/github.com/jake-scott/go-functional)

Test coverage is [available here](https://jake-scott.github.io/go-functional/coverage.html).

# License

This project is licensed under the Apache v2 License - see the
[LICENSE](LICENSE) file for details.

