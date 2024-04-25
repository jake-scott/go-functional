package channel_test

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/jake-scott/go-functional/iter/channel"
)

func ExampleIterator() {
	ctx := context.Background()

	// will always return the same output
	rand := rand.New(rand.NewSource(123))

	ch := make(chan int)
	go func() {
		for i := 0; i < 10; i++ {
			r := rand.Int() % 100

			ch <- r
		}

		close(ch)
	}()

	iter := channel.New(ch)
	for iter.Next(ctx) {
		fmt.Printf("item: %d\n", iter.Get(ctx))
	}

	if err := iter.Error(); err != nil {
		panic(err)
	}

	// output:
	// item: 89
	// item: 46
	// item: 43
	// item: 83
	// item: 87
	// item: 63
	// item: 48
	// item: 91
	// item: 75
	// item: 28
}
