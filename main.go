// Command unreturned reports loops that produce a value: extract as a function and return.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"strings"

	"blake.io/unreturned/internal/unreturned"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	os.Exit(run())
}

func run() int {
	log.SetFlags(0)
	opts, args, err := driverFlags(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	os.Args = append([]string{os.Args[0]}, args...)

	stop, err := startProfiles(opts)
	if err != nil {
		log.Fatal(err)
	}
	defer stop()

	ctx, task := trace.NewTask(context.Background(), "unreturned")
	defer task.End()

	if unreturned.CanRunSource(args) {
		code, err := unreturned.RunSource(ctx, os.Stderr, args)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return code
	}

	singlechecker.Main(unreturned.Analyzer)
	return 0
}

type profileOptions struct {
	tracePath string
	cpuPath   string
	memPath   string
}

func driverFlags(args []string) (profileOptions, []string, error) {
	var opts profileOptions
	rest := args[:0]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, value, ok := strings.Cut(arg, "=")
		switch name {
		case "-trace", "--trace":
			if !ok {
				i++
				if i >= len(args) {
					return opts, nil, flag.ErrHelp
				}
				value = args[i]
			}
			opts.tracePath = value
		case "-cpuprofile", "--cpuprofile":
			if !ok {
				i++
				if i >= len(args) {
					return opts, nil, flag.ErrHelp
				}
				value = args[i]
			}
			opts.cpuPath = value
		case "-memprofile", "--memprofile":
			if !ok {
				i++
				if i >= len(args) {
					return opts, nil, flag.ErrHelp
				}
				value = args[i]
			}
			opts.memPath = value
		default:
			rest = append(rest, arg)
		}
	}
	return opts, rest, nil
}

func startProfiles(opts profileOptions) (func(), error) {
	var cleanups []func()
	if opts.tracePath != "" {
		f, err := os.Create(opts.tracePath)
		if err != nil {
			return nil, err
		}
		if err := trace.Start(f); err != nil {
			f.Close()
			return nil, err
		}
		cleanups = append(cleanups, func() {
			trace.Stop()
			if err := f.Close(); err != nil {
				log.Print(err)
			}
		})
	}
	if opts.cpuPath != "" {
		f, err := os.Create(opts.cpuPath)
		if err != nil {
			return nil, err
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			f.Close()
			return nil, err
		}
		cleanups = append(cleanups, func() {
			pprof.StopCPUProfile()
			if err := f.Close(); err != nil {
				log.Print(err)
			}
		})
	}
	if opts.memPath != "" {
		runtime.MemProfileRate = 1
		cleanups = append(cleanups, func() {
			runtime.GC()
			f, err := os.Create(opts.memPath)
			if err != nil {
				log.Print(err)
				return
			}
			if err := pprof.WriteHeapProfile(f); err != nil {
				log.Print(err)
			}
			if err := f.Close(); err != nil {
				log.Print(err)
			}
		})
	}
	return func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}, nil
}
