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
 * This example demonstrates how resolve a set of IP addresses to hostnames
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

func toHostname(addr netip.Addr) (string, error) {
	hn := addr.String()

	names, err := net.LookupAddr(hn)
	if err == nil {
		hn = names[0]
	}

	time.Sleep(1 * time.Second)
	return hn, err
}

func main() {
	ch := make(chan netip.Addr)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// generate IP addresses in the background
	go generateIps(prefix, ch)

	stage := functional.NewChannelStage(ch,
		functional.InheritOptions(true),
		functional.WithTracing(true),
		functional.WithContext(ctx))

	results := functional.Map(stage, toHostname,
		functional.ProcessingType(functional.StreamingStage),
		functional.Parallelism(100))

	// it would be better to read from results.Iterator() directly, but
	// this demonstrates how a Reduce step can be used
	hosts := make([]string, 100)
	hosts = functional.Reduce(results, hosts, functional.SliceFromIterator)

	for _, hn := range hosts {
		fmt.Println(hn)
	}
}
