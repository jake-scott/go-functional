package channel

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var _channelInputTest1 []string = []string{
	"This is some test input with",
	"multipe lines",
	"in it and multiple words",
	"per line.",
}

func TestChannelIter(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	ch := make(chan string)
	go func() {
		for _, line := range _channelInputTest1 {
			ch <- line
		}

		close(ch)
	}()

	iter := New(ch)
	gotLines := []string{}

	// before a Next() call it should return the zero value..
	assert.Equal("", iter.Get())

	for iter.Next(ctx) {
		gotLines = append(gotLines, iter.Get())
	}

	assert.Equal(_channelInputTest1, gotLines)
	assert.Nil(iter.Error())
}

func TestChannelIteratorTimeout(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	ch := make(chan string)
	go func() {
		for _, line := range _channelInputTest1 {
			ch <- line
		}

		// Don't close ch, to cause a timeout
	}()

	iter := New(ch)
	ctx, cancel := context.WithTimeout(ctx, time.Millisecond*20) // only read the iterator for 20 ms total

	gotLines := []string{}
	defer cancel()
	for iter.Next(ctx) {
		gotLines = append(gotLines, iter.Get())
	}

	// the context should have timed out
	assert.NotNil(iter.Error())
	assert.ErrorIs(iter.Error(), context.DeadlineExceeded)

	// but also we should have received all the data
	assert.Equal(_channelInputTest1, gotLines)
}
