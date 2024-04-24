package functional

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWrapperUnordered(t *testing.T) {
	assert := assert.New(t)

	// make sure we can call this through the generic types
	var wr wrapItemFunc[string, string] = unorderedWrapper[string]
	var unw unwrapItemFunc[string, string] = unorderedUnwrapper[string]
	var sf switcherFunc[string, int, int] = unorderedSwitcher[string, int]

	// make a wrapped string item
	w := wr(123, "foo")
	assert.IsType("a string", w)
	assert.Equal("foo", w)

	// unwrap it..
	uw := unw(w)
	assert.IsType("a string", uw)
	assert.Equal("foo", uw)

	// switch the "wrapped" string for an int
	sw := sf(w, 456)
	assert.IsType(0, sw)
	assert.Equal(456, sw)
}

func TestWrapperOrdered(t *testing.T) {
	assert := assert.New(t)

	var wr wrapItemFunc[string, item[string]] = orderedWrapper[string]
	var unw unwrapItemFunc[item[string], string] = orderedUnwrapper[string]
	var sf switcherFunc[item[string], int, item[int]] = orderedSwitcher[string, int]

	// make a wrapped string item
	w := wr(123, "foo")
	assert.IsType(w, item[string]{})
	assert.Equal(item[string]{123, "foo"}, w)

	// unwrap it..
	uw := unw(w)
	assert.IsType("a string", uw)
	assert.Equal("foo", uw)

	// switch the "wrapped" string for an int
	sw := sf(w, 456)
	assert.IsType(sw, item[int]{})
	assert.Equal(item[int]{123, 456}, sw)
}
