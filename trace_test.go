package functional

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTracer(t *testing.T) {
	assert := assert.New(t)

	var buf string
	testTracer := func(format string, v ...any) {
		buf = fmt.Sprintf("XXX "+format, v...)
	}
	DefaultTracer = func(format string, v ...any) {
		buf = fmt.Sprintf("YYY "+format, v...)
	}

	// test tracing using the default tracer function
	tr := NewTracer(123, "testDefaultTracer", nil)

	assert.Contains(buf, "YYY")
	assert.Contains(buf, "START [stage #123] testDefaultTracer")

	tr2 := tr.SubTracer("ZZZ %d", 321)
	assert.Contains(buf, "ZZZ 321")
	assert.Contains(buf, "START [stage #123.1] testDefaultTracer")

	tr3 := tr.SubTracer("PPP %d", 333)
	assert.Contains(buf, "PPP 333")
	assert.Contains(buf, "START [stage #123.2] testDefaultTracer")

	tr4 := tr3.SubTracer("QQQ %d", 666)
	assert.Contains(buf, "QQQ 666")
	assert.Contains(buf, "START [stage #123.2.1] testDefaultTracer")

	tr4.End()
	tr3.End()
	tr2.End()

	tr.End()
	assert.Contains(buf, "YYY")
	assert.Contains(buf, "END [stage #123] testDefaultTracer")

	// test tracing using a supplied tracing function
	tr = NewTracer(456, "testMyTracer", testTracer)
	assert.Contains(buf, "XXX")
	assert.Contains(buf, "START [stage #456] testMyTracer")

	tr.End()
	assert.Contains(buf, "XXX")
	assert.Contains(buf, "END [stage #456] testMyTracer")
}
