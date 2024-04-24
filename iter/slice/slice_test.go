package slice_test

import (
	"context"
	"testing"

	"github.com/jake-scott/go-functional"
	"github.com/jake-scott/go-functional/iter/slice"
	"github.com/stretchr/testify/assert"
)

var _sliceInputTest1 []string = []string{
	"This is some test input with",
	"multipe lines",
	"in it and multiple words",
	"per line.",
}

func TestSliceIter(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	iter := slice.New(_sliceInputTest1)

	gotLines := []string{}
	for iter.Next(ctx) {
		gotLines = append(gotLines, iter.Get(ctx))
	}

	assert.Equal(_sliceInputTest1, gotLines)
	assert.Nil(iter.Error())

	// test that we can assert to a SizeHint via the Iterator interface
	var iterInt functional.Iterator[string] = &iter
	sh, ok := iterInt.(functional.Size[string])
	assert.True(ok)

	// .. and that SizeHint.Size() returns the right number
	assert.Equal(uint(4), sh.Size())
}

// Test with an empty slice
func TestSliceIter2(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	iter := slice.New([]int(nil))

	count := 0
	for iter.Next(ctx) {
		count++
	}

	assert.Equal(count, 0)
	assert.Nil(iter.Error())

	// Zero value
	assert.Equal(iter.Get(ctx), 0)
}
