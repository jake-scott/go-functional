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
	tr := newTracer(123, "testDefaultTracer", nil)

	assert.Contains(buf, "YYY")
	assert.Contains(buf, "START [stage #123] testDefaultTracer")

	tr2 := tr.subTracer("ZZZ %d", 321)
	assert.Contains(buf, "ZZZ 321")
	assert.Contains(buf, "START [stage #123.1] testDefaultTracer")

	tr3 := tr.subTracer("PPP %d", 333)
	assert.Contains(buf, "PPP 333")
	assert.Contains(buf, "START [stage #123.2] testDefaultTracer")

	tr4 := tr3.subTracer("QQQ %d", 666)
	assert.Contains(buf, "QQQ 666")
	assert.Contains(buf, "START [stage #123.2.1] testDefaultTracer")

	tr4.end()
	tr3.end()
	tr2.end()

	tr.end()
	assert.Contains(buf, "YYY")
	assert.Contains(buf, "END [stage #123] testDefaultTracer")

	// test tracing using a supplied tracing function
	tr = newTracer(456, "testMyTracer", testTracer)
	assert.Contains(buf, "XXX")
	assert.Contains(buf, "START [stage #456] testMyTracer")

	tr.end()
	assert.Contains(buf, "XXX")
	assert.Contains(buf, "END [stage #456] testMyTracer")
}
