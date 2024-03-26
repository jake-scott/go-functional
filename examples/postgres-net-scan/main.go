package main

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync/atomic"
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

// Find hosts listening on port 5432
func pingPostgres(addr netip.Addr) (bool, error) {
	a := addr.String() + ":5432"

	con, err := net.DialTimeout("tcp", a, time.Second*timeout)
	if err != nil {
		return false, err
	}

	con.Close()
	return true, nil
}

// Find hosts listening on port 5432
func pingSsh(addr netip.Addr) (bool, error) {
	a := addr.String() + ":22"

	con, err := net.DialTimeout("tcp", a, time.Second*timeout)
	if err != nil {
		return false, err
	}

	con.Close()
	return true, nil
}

func toHostname(addr netip.Addr) (string, error) {

	hn := addr.String()

	names, err := net.LookupAddr(hn)
	if err == nil {
		hn = names[0]
	}

	return hn, err
}

type myError struct {
	err error
}

func main() {
	ch := make(chan netip.Addr)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// generate IP addresses in the background
	go generateIps(prefix, ch)

	noError := myError{err: nil}
	var firstError atomic.Value
	firstError.Store(noError)

	errHandler := func(ec functional.ErrorContext, err error) bool {
		firstError.CompareAndSwap(noError, myError{err})
		return true
	}

	result := functional.NewChannelStage(ch,
		functional.WithErrorHandler(errHandler),
		functional.WithContext(ctx),
		functional.InheritOptions(true),
		functional.WithTracing(true),
		functional.ProcessingType(functional.BatchStage),
		functional.SizeHint(256)).
		Filter(pingPostgres, functional.Parallelism(100), functional.InheritOptions(true)).
		Filter(pingSsh, functional.SizeHint(10))

	result2 := functional.Map(result, toHostname, functional.WithTracing(true), functional.SizeHint(10))

	fe := firstError.Load().(myError)
	if fe.err != nil {
		fmt.Printf("Pipeline error: %s\n", fe.err)
	}

	iter := result2.Iterator()
	countAlive := 0
	for iter.Next(ctx) {
		fmt.Printf("Alive: %s\n", iter.Get())
		countAlive++
	}

	if err := iter.Error(); err != nil {
		panic(err)
	}

	fmt.Printf("Found %d postgres/ssh hosts out of %d\n", countAlive, countTotal)
}
