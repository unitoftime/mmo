package main

import (
	"os"
	"fmt"
	"runtime"
	"runtime/pprof"
	"flag"

	"github.com/unitoftime/mmo/app/client"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

func main() {
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			panic(fmt.Sprintf("could not create CPU profile: %v", err))
		}
		defer f.Close() // error handling omitted for example
		if err := pprof.StartCPUProfile(f); err != nil {
			panic(fmt.Sprintf("could not start CPU profile: %v", err))
		}
		defer pprof.StopCPUProfile()
	}

	// TODO - catch panics, exits and finish exporting mem and cpu prof
	client.Main(client.Config{
		ProxyUri: "webrtc://localhost:7777",
		// ProxyUri: "wss://localhost:7777",
		// ProxyUri: "wss://mmo.unit.dev:443",
		Test: true,
	})

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			panic(fmt.Sprintf("could not create memory profile: %v", err))
		}
		defer f.Close() // error handling omitted for example
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			panic(fmt.Sprintf("could not write memory profile: %v", err))
		}
	}
}
