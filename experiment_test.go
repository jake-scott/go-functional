package functional

import (
	"fmt"
	"testing"
)

// func TestOne(t *testing.T) {
// 	s := []string{"one", "two", "three"}

// 	si := NewSliceIterator(s)
// 	iter := Iterator[string](&si)

// 	for iter.Next() {
// 		t.Logf("Item: %v", iter.Get())
// 	}

// 	t.Logf("Bad: %v", iter.Get())
// }

// func TestTwo(t *testing.T) {
// 	s := []string{"one", "two", "three", "12345", "abcde"}

// 	gen := NewBatchSliceGenerator(s)
// 	result := gen.Filter(is5letter).
// 		Filter(startsLetter).
// 		Iterator()

// 	for result.Next() {
// 		t.Logf("Item2: %v", result.Get())
// 	}
// }

// func TestThree(t *testing.T) {
// 	s := []string{"one", "two", "three", "12345", "abcde"}
// 	ch := make(chan string)
// 	go func() {
// 		for _, item := range s {
// 			ch <- item
// 		}

// 		close(ch)
// 	}()

// 	si := NewChannelIterator(ch)
// 	iter := Iterator[string](&si)

// 	for iter.Next() {
// 		t.Logf("Item: %v", iter.Get())
// 	}

// 	t.Logf("Bad: %v", iter.Get())
// }

// func TestFour(t *testing.T) {
// 	s := []string{"one", "two", "three", "12345", "abcde"}
// 	ch := make(chan string)
// 	go func() {
// 		for _, item := range s {
// 			ch <- item
// 		}

// 		close(ch)
// 	}()

// 	gen := NewBatchChannelGenerator(ch)
// 	result := gen.Filter(is5letter).
// 		Filter(startsLetter).
// 		Iterator()

// 	for result.Next() {
// 		t.Logf("Item2: %v", result.Get())
// 	}
// }

// func TestFive(t *testing.T) {
// 	s := []string{"one", "two", "three", "12345", "abcde"}

// 	gen := NewStreamingSliceGenerator(s)
// 	result := gen.Filter(is5letter, OutputBatch).
// 		Filter(startsLetter, OutputBatch).Iterator()

// 	for result.Next() {
// 		t.Logf("Item2: %v", result.Get())
// 	}
// }

func TestFilter1(t *testing.T) {
	s := []string{"one", "two", "three", "12345", "abcde"}

	opts1 := []GeneratorOption[string]{}
	opts1 = append(opts1, ProcessingType[string](Batch))
	//	opts1 = append(opts1, Parallelism[string](5))
	opts1 = append(opts1, InheritOptions[string](true))

	opts2 := []GeneratorOption[string]{}
	//	opts2 = append(opts2, Parallelism[string](2))

	result := NewSliceGenerator(s, opts1...).
		Filter(is5letter, opts2...).
		Filter(startsLetter).
		Iterator()
	for result.Next() {
		t.Logf("Item: %v", result.Get())
	}
}

type I1 interface {
	Foo()
}

type I2 interface {
	Bar()
}

type s1 string
type s2 string

func (s s1) Foo() {
	fmt.Printf("Foo s1: %s\n", s)
}
func (s s2) Foo() {
	fmt.Printf("Foo s2: %s\n", s)
}
func (s s2) Bar() {
	fmt.Printf("Bar s2: %s\n", s)
}

func x(s I1) {
	s.Foo()

	b, ok := s.(I2)
	if ok {
		b.Bar()
	}
}
func TestX(t *testing.T) {
	var one s1 = "test1"
	x(one)
	var two s2 = "test1"
	x(two)
}
