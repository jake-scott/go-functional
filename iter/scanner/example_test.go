package scanner_test

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/jake-scott/go-functional/iter/scanner"
)

func ExampleIterator() {
	f, err := os.Open("/etc/passwd")
	if err != nil {
		panic(err)
	}

	s := bufio.NewScanner(f)
	ctx := context.Background()
	iter := scanner.New(s)

	for iter.Next(ctx) {
		fmt.Printf("Line: <%s>\n", iter.Get())
	}

	if err := iter.Error(); err != nil {
		panic(err)
	}
}
