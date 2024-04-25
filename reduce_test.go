package functional

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const sumHundredInts = 4682

func TestReduceOO(t *testing.T) {
	assert := assert.New(t)

	add := func(a, i int) int {
		return a + i
	}

	// test starting at zero
	result := NewSliceStage(hundredInts).Reduce(0, add)
	assert.Equal(sumHundredInts, result)

	// test starting at a non-zero value
	result2 := NewSliceStage(hundredInts).Reduce(123, add)
	assert.Equal(sumHundredInts+123, result2)
}

func TestReduceProc(t *testing.T) {
	assert := assert.New(t)

	// example of a reduce func that returns a different type than the
	// stage item type
	add := func(a float32, i int) float32 {
		return a + float32(i)
	}

	// test starting at zero
	stage := NewSliceStage(hundredInts)
	result := Reduce(stage, 0, add)
	assert.IsType(float32(0), result)
	assert.Equal(float32(sumHundredInts), result)

	// test starting at a non-zero value
	stage2 := NewSliceStage(hundredInts)
	result2 := Reduce(stage2, 123, add)
	assert.IsType(float32(0), result2)
	assert.Equal(float32(sumHundredInts+123), result2)
}
