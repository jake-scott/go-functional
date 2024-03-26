package slice_test

import (
	"context"
	"fmt"

	"github.com/jake-scott/go-functional/iter/slice"
)

func ExampleIterator() {
	input := []string{"dog", "cat", "fox", "pigeon"}

	ctx := context.Background()
	iter := slice.New(input)

	for iter.Next(ctx) {
		fmt.Printf("Animal: <%s>\n", iter.Get())
	}

	if err := iter.Error(); err != nil {
		panic(err)
	}

	// output:
	// Animal: <dog>
	// Animal: <cat>
	// Animal: <fox>
	// Animal: <pigeon>
}
