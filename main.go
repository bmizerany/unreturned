// Command unreturned reports loops that produce a value: extract as a function and return.
package main

import (
	"blake.io/unreturned/internal/unreturned"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(unreturned.Analyzer)
}
