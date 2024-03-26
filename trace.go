package functional

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type tracer interface {
	subTracer(description string, v ...any) tracer
	msg(format string, v ...any)
	end()
}

// TraceFunc defines the function prototype of a tracing function
// Per stage functions can be configured using WithTraceFunc
type TraceFunc func(format string, v ...any)

// DefaultTracer is the global default trace function.  It prints messages to
// stderr.  DefaultTracer can be replaced by another tracing function to effect
// all stages.
var DefaultTracer = func(format string, v ...any) {
	fmt.Fprintf(os.Stderr, "<TRACE> "+format+"\n", v...)
}

type realTracer struct {
	begin       time.Time
	description string
	ids         []uint32
	subids      atomic.Uint32
	traceFunc   TraceFunc
}

// newTracer creates a new tracer with a given ID and description.  If
// the tracefunc f is nil, DefaultTracer is used to process trace calls.
// The optional parameters v are used as fmt.Printf parameters to format
// the description.
// Usually one tracer will be created for a transation, and sub-routines will
// create new tracers with SubTracer().
//
// Example:
//
//	parentTracer := newTracer(1, "parent", nil)
func newTracer(id uint32, description string, f TraceFunc, v ...any) *realTracer {
	if f == nil {
		f = DefaultTracer
	}
	now := time.Now()

	description = fmt.Sprintf(description, v...)

	t := &realTracer{
		begin:       now,
		description: description,
		ids:         []uint32{id},
		traceFunc:   f,
	}

	t.start()
	return t
}

func (t *realTracer) id() string {
	idStrings := make([]string, len(t.ids))
	for i, n := range t.ids {
		idStrings[i] = strconv.Itoa(int(n))
	}
	return strings.Join(idStrings, ".")
}

func (t *realTracer) start() {
	t.begin = time.Now()
	t.traceFunc("%s: START [stage #%s] %s", t.begin.Format(time.RFC3339), t.id(), t.description)
}

// subTracer returns a new tracer based on t, with a new sub ID
// description is formatted with the optional v parameters, and
// added to the description of the parent.
//
// Example:
//
//	childTracer1 := parentTracer.subTracer("child %d", 1)
//	childTracer2 := parentTracer.subTracer("child %d", 2)
//	childTracer2a := chileTracer2.subTracer("grandchild %d", 1)
func (t *realTracer) subTracer(description string, v ...any) tracer {
	subId := t.subids.Add(1)

	t2 := *t
	t2.subids = atomic.Uint32{}
	t2.ids = append(slices.Clone(t.ids), subId)
	t2.description += fmt.Sprintf(" / "+description, v...)

	t2.start()
	return &t2
}

// msg
func (t *realTracer) msg(format string, v ...any) {
	var args []any = []any{
		time.Now().Format(time.RFC3339), t.id(), t.description,
	}
	args = append(args, v...)
	t.traceFunc("%s: MSG [stage #%s] %s: "+format, args...)
}

func (t *realTracer) end() {
	t.traceFunc("%s: END [stage #%s] %s", time.Now().Format(time.RFC3339), t.id(), t.description)
}

type nullTracer struct{}

func (t nullTracer) subTracer(description string, v ...any) tracer { return t }
func (t nullTracer) msg(string, ...any)                            {}
func (t nullTracer) end()                                          {}
