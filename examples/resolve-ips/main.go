package main

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/jake-scott/go-functional"
)

/*
 * This example demonstrates how to use a parallel filter to find Postgres
 * servers on a network that are also running an SSH server.
 *
 * The input is generated and piped to a channel that the initial stage reads
 * from.  Two filters are then used - one to find hosts responding on port
 * 5432 and then another to find the subset of those responding on port 22.
 *
 * Results are streamed between stages so that the scan for port 22 happens
 * at the same time as the scan for port 5432 is going on, as the first stage
 * produces its results.
 */

const prefix = "10.219.224.0/24"
const timeout = 5

var countTotal int

// Generate a stream of IP addresses given a CIDR range
func generateIps(cidr string, ch chan netip.Addr) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		panic(err)
	}

	go func() {
		for addr := prefix.Addr(); prefix.Contains(addr); addr = addr.Next() {
			ch <- addr
			countTotal++
		}

		close(ch)
	}()
}

func toHostname(addr netip.Addr) string {
	hn := addr.String()

	names, err := net.LookupAddr(hn)
	if err == nil {
		hn = names[0]
	}

	time.Sleep(1 * time.Second)
	return hn
}

func main() {
	ch := make(chan netip.Addr)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// generate IP addresses in the background
	go generateIps(prefix, ch)

	stage := functional.NewChannelStage(ch,
		functional.InheritOptions(true),
		functional.WithTracing(true))

	results := functional.Map(stage, toHostname,
		functional.ProcessingType(functional.StreamingStage),
		functional.Parallelism(10))

	iter := results.Iterator()
	for iter.Next(ctx) {
		fmt.Println(iter.Get(ctx))
	}

	if err := iter.Error(); err != nil {
		panic(err)
	}

}
