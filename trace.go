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

type Tracer interface {
	SubTracer(description string, v ...any) Tracer
	Msg(format string, v ...any)
	End()
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

type tracer struct {
	begin       time.Time
	description string
	ids         []uint32
	subids      atomic.Uint32
	traceFunc   TraceFunc
}

func NewTracer(id uint32, description string, f TraceFunc, v ...any) *tracer {
	if f == nil {
		f = DefaultTracer
	}
	now := time.Now()

	description = fmt.Sprintf(description, v...)

	t := &tracer{
		begin:       now,
		description: description,
		ids:         []uint32{id},
		traceFunc:   f,
	}

	t.start()
	return t
}

func (t *tracer) id() string {
	idStrings := make([]string, len(t.ids))
	for i, n := range t.ids {
		idStrings[i] = strconv.Itoa(int(n))
	}
	return strings.Join(idStrings, ".")
}

func (t *tracer) start() {
	t.begin = time.Now()
	t.traceFunc("%s: START [stage #%s] %s", t.begin.Format(time.RFC3339), t.id(), t.description)
}

func (t *tracer) SubTracer(description string, v ...any) Tracer {
	subId := t.subids.Add(1)

	t2 := *t
	t2.subids = atomic.Uint32{}
	t2.ids = append(slices.Clone(t.ids), subId)
	t2.description += fmt.Sprintf(" / "+description, v...)

	t2.start()
	return &t2
}

func (t *tracer) Msg(format string, v ...any) {
	var args []any = []any{
		time.Now().Format(time.RFC3339), t.id(), t.description,
	}
	args = append(args, v...)
	t.traceFunc("%s: MSG [stage #%s] %s: "+format, args...)
}

func (t *tracer) End() {
	t.traceFunc("%s: END [stage #%s] %s", time.Now().Format(time.RFC3339), t.id(), t.description)
}

type NullTracer struct{}

func (t NullTracer) SubTracer(description string, v ...any) Tracer { return t }
func (t NullTracer) Msg(string, ...any)                            {}
func (t NullTracer) End()                                          {}
