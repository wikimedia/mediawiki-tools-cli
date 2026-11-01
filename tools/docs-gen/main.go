// Command docs-gen generates the static command manual from Cobra help.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	output := flag.String("output", "website/src/content/docs/manual", "Generated manual directory")
	flag.Parse()
	if err := generateManual(*output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
