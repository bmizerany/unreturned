// Command unreturned reports loop-assigned results read after the loop exits.
package main

import (
	"blake.io/unreturned/internal/unreturned"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(unreturned.Analyzer)
}
