package scanner

import (
	"bufio"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var _scanInputTest1 string = `This is some test input with
multipe lines
in it and multiple words
per line.`

func TestScannerIter(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	s := bufio.NewScanner(strings.NewReader(_scanInputTest1))
	s.Split(bufio.ScanLines)

	iter := New(s)

	// Get should return the zero value until we call Next
	x := iter.Get()
	assert.Equalf("", x, "Expected string zero value")

	gotLines := []string{}
	for iter.Next(ctx) {
		gotLines = append(gotLines, iter.Get())
	}

	wantLines := strings.Split(_scanInputTest1, "\n")

	assert.Equal(wantLines, gotLines)
	assert.Nil(iter.Error())
}

// Scanner wrapper that always panncs in Scan()
type panicScanner struct {
	bufio.Scanner
}

func (thing *panicScanner) Scan() bool {
	panic("FOO FOO FOO")
}
func TestScannerIterPanic(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	thing := bufio.NewScanner(strings.NewReader(_scanInputTest1))
	foo := panicScanner{Scanner: *thing}

	iter := New(&foo)

	nGood := 0
	// should panic and return false
	for iter.Next(ctx) {
		nGood++
	}

	assert.Equalf(0, nGood, "zero good calls to Next() expected")

	// should be an error
	assert.NotNil(iter.Error())

	// that should be our error from catching the panic
	assert.IsType(iter.Error(), ErrTooManyTokens{})

	assert.Contains(iter.Error().Error(), "too many tokens")
	assert.Contains(iter.Error().Error(), "FOO FOO FOO")
}

func TestScannerCancelled(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := bufio.NewScanner(strings.NewReader(_scanInputTest1))
	s.Split(bufio.ScanLines)

	iter := New(s)

	gotLines := []string{}
	for iter.Next(ctx) {
		gotLines = append(gotLines, iter.Get())
		cancel()
	}

	wantLines := (strings.Split(_scanInputTest1, "\n"))[0:1]
	assert.Equal(wantLines, gotLines)

	assert.NotNil(iter.Error())
	assert.ErrorIs(context.Canceled, iter.Error())
}

func TestTooManyTokensError(t *testing.T) {
	assert := assert.New(t)
	var myError = errors.New("my test error")

	e1 := ErrTooManyTokens{panicMessage: "this is e1"}
	e2 := ErrTooManyTokens{err: myError}

	assert.Contains(e1.Error(), "too many tokens")

	assert.Contains(e2.Error(), "too many tokens")
	assert.Contains(e2.Error(), "my test error")
	assert.ErrorIs(e2, myError)
}
