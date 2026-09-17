package functional

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type chainPerson struct {
	name string
	age  int
}

// TestMethodChainingAcrossTypes exercises the generic Map[M] and Reduce[A]
// methods, which allow the element type to change at each step of a chain.
func TestMethodChainingAcrossTypes(t *testing.T) {
	people := []chainPerson{{"alice", 31}, {"bob", 22}, {"carol", 45}}

	got := NewSliceStage(people).
		Filter(func(p chainPerson) (bool, error) { return p.age > 25, nil }).
		Map(func(p chainPerson) (string, error) { return strings.ToUpper(p.name), nil }).
		Map(func(s string) (int, error) { return len(s), nil }).
		Reduce("", func(acc string, n int) (string, error) { return acc + strconv.Itoa(n) + ",", nil })
	assert.Equal(t, "5,5,", got)

	// same-type map still infers M == T
	same := NewSliceStage([]int{1, 2, 3}).
		Map(func(i int) (int, error) { return i * 2, nil }).
		Reduce(0, func(a, i int) (int, error) { return a + i, nil })
	assert.Equal(t, 12, same)

	// ordered parallel batch through the method form
	ordered := NewSliceStage([]int{3, 1, 2}, InheritOptions(true), Parallelism(4), PreserveOrder(true)).
		Map(func(i int) (string, error) { return strconv.Itoa(i), nil }).
		Reduce("", func(a, s string) (string, error) { return a + s, nil })
	assert.Equal(t, "312", ordered)

	// streaming through the method form
	streamed := NewSliceStage([]int{1, 2, 3}, InheritOptions(true), ProcessingType(StreamingStage)).
		Map(func(i int) (float64, error) { return float64(i) / 2, nil }).
		Reduce(0.0, func(a, f float64) (float64, error) { return a + f, nil })
	assert.Equal(t, 3.0, streamed)

	// method values require explicit instantiation
	toStr := NewSliceStage([]int{7}).Map[string]
	strs := toStr(func(i int) (string, error) { return strconv.Itoa(i), nil })
	assert.Equal(t, "7", strs.Reduce("", func(a, s string) (string, error) { return a + s, nil }))
}
