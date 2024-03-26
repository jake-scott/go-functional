package functional

import (
	"math/rand"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func isEven(i int) bool {
	return i%2 == 0
}

func is5letter(s string) bool {
	return len(s) == 5
}

func startsLetter(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsLetter(r)
}

func slowFilter[T any](d time.Duration, f FilterFunc[T]) FilterFunc[T] {
	return func(t T) bool {
		time.Sleep(d)
		return f(t)
	}
}

func TestFilterInts(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		f     FilterFunc[int]
		want  []int
	}{
		{
			name:  "find even ints from list of 8",
			input: []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
			f:     isEven,
			want:  []int{2, 4, 6, 8},
		},
		{
			name:  "find even ints from only odds",
			input: []int{1, 3, 5, 7, 9},
			f:     isEven,
			want:  []int{},
		},
		{
			name:  "empty list",
			input: []int{},
			f:     isEven,
			want:  []int{},
		},
		{
			name:  "null list",
			input: nil,
			f:     isEven,
			want:  []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := Filter(tt.input, tt.f)
			assert.EqualValues(t, tt.want, filtered)
		})
	}
}

func TestFilterStrings(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		f     FilterFunc[string]
		want  []string
	}{
		{
			name:  "find 5 letter words",
			input: []string{"cat", "aloud", "whoop", "dog", "horse"},
			f:     is5letter,
			want:  []string{"aloud", "whoop", "horse"},
		},
		{
			name:  "empty list",
			input: []string{},
			f:     is5letter,
			want:  []string{},
		},
		{
			name:  "null list",
			input: nil,
			f:     is5letter,
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := Filter(tt.input, tt.f)
			assert.EqualValues(t, tt.want, filtered)
		})
	}
}

func makeInts(size int) []int {
	out := make([]int, size)
	for i, _ := range out {
		out[i] = i
	}

	return out
}

func makeStrings(size int) []string {
	out := make([]string, size)
	for i, _ := range out {

		sz := rand.Int() % 20
		s := strings.Repeat("A", sz)
		out[i] = s
	}

	return out
}

func BenchmarkFilterInts(b *testing.B) {
	isEven5ns := slowFilter(time.Nanosecond*5, isEven)
	isEven5µs := slowFilter(time.Microsecond*5, isEven)
	isEven5ms := slowFilter(time.Millisecond*5, isEven)

	tests := []struct {
		name        string
		size        int
		f           FilterFunc[int]
		parallelism uint
	}{
		// single threaded, fast as possible filter
		{"1 element fast filter", 1, isEven, 1},
		{"10^2 element fast filter", 100, isEven, 1},
		{"10^3 element fast filter", 1000, isEven, 1},
		{"10^4 element fast filter", 10000, isEven, 1},
		{"10^5 element fast filter", 100000, isEven, 1},
		{"10^6 element fast filter", 1000000, isEven, 1},
		// single threaded, filter taking 5ns
		{"1 element 5ns filter", 1, isEven5ns, 1},
		{"10^2 element 5ns filter", 100, isEven5ns, 1},
		{"10^3 element 5ns filter", 1000, isEven5ns, 1},
		{"10^4 element 5ns filter", 10000, isEven5ns, 1},
		{"10^5 element 5ns filter", 100000, isEven5ns, 1},
		{"10^6 element 5ns filter", 1000000, isEven5ns, 1},
		// single threaded, filter taking 5µs
		{"1 element 5µs filter", 1, isEven5µs, 1},
		{"10^2 element 5µs filter", 100, isEven5µs, 1},
		{"10^3 element 5µs filter", 1000, isEven5µs, 1},
		{"10^4 element 5µs filter", 10000, isEven5µs, 1},
		// single threaded, filter taking 5ms
		{"1 element 5ms filter", 1, isEven5ms, 1},
		{"10^2 element 5ms filter", 100, isEven5ms, 1},
		{"10^3 element 5ms filter", 1000, isEven5ms, 1},
		{"10^4 element 5ms filter", 10000, isEven5ms, 1},
		// 100 threads, fast as possible filter
		{"1 element fast filter x 100", 1, isEven, 100},
		{"10^2 element fast filter x 100", 100, isEven, 100},
		{"10^3 element fast filter x 100", 1000, isEven, 100},
		{"10^4 element fast filter x 100", 10000, isEven, 100},
		{"10^5 element fast filter x 100", 100000, isEven, 100},
		{"10^6 element fast filter x 100", 1000000, isEven, 100},
		// 100 threads, filter taking 5ns
		{"1 element 5ns filter x 100", 1, isEven5ns, 100},
		{"10^2 element 5ns filter x 100", 100, isEven5ns, 100},
		{"10^3 element 5ns filter x 100", 1000, isEven5ns, 100},
		{"10^4 element 5ns filter x 100", 10000, isEven5ns, 100},
		{"10^5 element 5ns filter x 100", 100000, isEven5ns, 100},
		{"10^6 element 5ns filter x 100", 1000000, isEven5ns, 100},
		// 100 threads, filter taking 5µs
		{"1 element 5µs filter x 100", 1, isEven5µs, 100},
		{"10^2 element 5µs filter x 100", 100, isEven5µs, 100},
		{"10^3 element 5µs filter x 100", 1000, isEven5µs, 100},
		{"10^4 element 5µs filter x 100", 10000, isEven5µs, 100},
		// 100 threads, filter taking 5ms
		{"1 element 5ms filter x 100", 1, isEven5ms, 100},
		{"10^2 element 5ms filter x 100", 100, isEven5ms, 100},
		{"10^3 element 5ms filter x 100", 1000, isEven5ms, 100},
		{"10^4 element 5ms filter x 100", 10000, isEven5ms, 100},
	}

	for _, tt := range tests {
		input := makeInts(tt.size)
		b.Run(tt.name, func(b *testing.B) {

			for n := 0; n < b.N; n++ {
				opt := FilterOptionParallelism(tt.parallelism)
				_ = Filter(input, tt.f, opt)
			}
		})
	}
}

func BenchmarkFilterStrings(b *testing.B) {
	is5Letter5ns := slowFilter(time.Nanosecond*5, is5letter)
	is5Letter5µs := slowFilter(time.Microsecond*5, is5letter)
	is5Letter5ms := slowFilter(time.Millisecond*5, is5letter)

	tests := []struct {
		name        string
		size        int
		f           FilterFunc[string]
		parallelism uint
	}{
		// single threaded, fast as possible filter
		{"1 element fast filter", 1, is5letter, 1},
		{"10^1 element fast filter", 100, is5letter, 1},
		{"10^2 element fast filter", 100, is5letter, 1},
		{"10^3 element fast filter", 1000, is5letter, 1},
		{"10^4 element fast filter", 10000, is5letter, 1},
		{"10^5 element fast filter", 100000, is5letter, 1},
		{"10^6 element fast filter", 1000000, is5letter, 1},
		// single threaded, filter taking 5ns
		{"1 element 5ns filter", 1, is5Letter5ns, 1},
		{"10^2 element 5ns filter", 100, is5Letter5ns, 1},
		{"10^3 element 5ns filter", 1000, is5Letter5ns, 1},
		{"10^4 element 5ns filter", 10000, is5Letter5ns, 1},
		{"10^5 element 5ns filter", 100000, is5Letter5ns, 1},
		{"10^6 element 5ns filter", 1000000, is5Letter5ns, 1},
		// single threaded, filter taking 5µs
		{"1 element 5µs filter", 1, is5Letter5µs, 1},
		{"10^2 element 5µs filter", 100, is5Letter5µs, 1},
		{"10^3 element 5µs filter", 1000, is5Letter5µs, 1},
		{"10^4 element 5µs filter", 10000, is5Letter5µs, 1},
		// single threaded, filter taking 5ms
		{"1 element 5ms filter", 1, is5Letter5ms, 1},
		{"10^2 element 5ms filter", 100, is5Letter5ms, 1},
		{"10^3 element 5ms filter", 1000, is5Letter5ms, 1},
		{"10^4 element 5ms filter", 10000, is5Letter5ms, 1},
		// 100 threads, fast as possible filter
		{"1 element fast filter x 100", 1, is5letter, 100},
		{"10^2 element fast filter x 100", 100, is5letter, 100},
		{"10^3 element fast filter x 100", 1000, is5letter, 100},
		{"10^4 element fast filter x 100", 10000, is5letter, 100},
		{"10^5 element fast filter x 100", 100000, is5letter, 100},
		{"10^6 element fast filter x 100", 1000000, is5letter, 100},
		// 100 threads, filter taking 5ns
		{"1 element 5ns filter x 100", 1, is5Letter5ns, 100},
		{"10^2 element 5ns filter x 100", 100, is5Letter5ns, 100},
		{"10^3 element 5ns filter x 100", 1000, is5Letter5ns, 100},
		{"10^4 element 5ns filter x 100", 10000, is5Letter5ns, 100},
		{"10^5 element 5ns filter x 100", 100000, is5Letter5ns, 100},
		{"10^6 element 5ns filter x 100", 1000000, is5Letter5ns, 100},
		// 100 threads, filter taking 5µs
		{"1 element 5µs filter x 100", 1, is5Letter5µs, 100},
		{"10^2 element 5µs filter x 100", 100, is5Letter5µs, 100},
		{"10^3 element 5µs filter x 100", 1000, is5Letter5µs, 100},
		{"10^4 element 5µs filter x 100", 10000, is5Letter5µs, 100},
		// 100 threads, filter taking 5ms
		{"1 element 5ms filter x 100", 1, is5Letter5ms, 100},
		{"10^2 element 5ms filter x 100", 100, is5Letter5ms, 100},
		{"10^3 element 5ms filter x 100", 1000, is5Letter5ms, 100},
	}

	for _, tt := range tests {
		input := makeStrings(tt.size)
		b.Run(tt.name, func(b *testing.B) {

			for n := 0; n < b.N; n++ {
				opt := FilterOptionParallelism(tt.parallelism)
				_ = Filter(input, tt.f, opt)
			}
		})
	}
}

func domainName(s string) string {
	parts := strings.SplitN(s, "@", 2)

	if len(parts) > 1 {
		return parts[1]
	}

	return ""
}

func TestMapStrings(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		f     MapFunc[string]
		want  []string
	}{
		{
			name:  "get domain names of email addresses",
			input: []string{"foo@bar.com", "test@testing.testing.baz"},
			f:     domainName,
			want:  []string{"bar.com", "testing.testing.baz"},
		},
		{
			name:  "find domain name form bad input",
			input: []string{"bad1", "not an email"},
			f:     domainName,
			want:  []string{"", ""},
		},
		{
			name:  "empty list",
			input: []string{},
			f:     domainName,
			want:  []string{},
		},
		{
			name:  "null list",
			input: nil,
			f:     domainName,
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped := Map(tt.input, tt.f)
			assert.EqualValues(t, tt.want, mapped)
		})
	}
}

type thing struct {
	num       int
	multipler int
}

func thingReducer(a, b thing) thing {
	return thing{
		num: a.num*b.multipler + b.num,
	}
}

func TestReduceThings(t *testing.T) {
	tests := []struct {
		name  string
		input []thing
		f     ReduceFunc[thing]
		want  thing
	}{
		{
			name:  "reduce list of things",
			input: []thing{{1, 3}, {5, 2}, {15, 4}, {4, 2}},
			f:     thingReducer,
			want:  thing{90, 0},
		},
		{
			name:  "one element",
			input: []thing{{1, 3}},
			f:     thingReducer,
			want:  thing{1, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reduced := Reduce(tt.input[1:], tt.input[0], tt.f)
			assert.Equal(t, tt.want, reduced)
		})
	}
}
