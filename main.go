// Command unreturned reports loops that produce a value: extract as a function and return.
package main

import (
	"fmt"
	"log"
	"os"

	"blake.io/unreturned/internal/unreturned"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	log.SetFlags(0)
	args := os.Args[1:]
	if unreturned.CanRunSource(args) {
		code, err := unreturned.RunSource(os.Stderr, args)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(code)
	}

	singlechecker.Main(unreturned.Analyzer)
}
