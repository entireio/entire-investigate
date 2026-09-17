// entire-investigate is an Entire CLI external command.
//
// Once built as an executable named `entire-investigate` on $PATH, the parent
// Entire CLI dispatches it when a user runs `entire investigate`.
package main

import (
	"fmt"
	"os"

	"github.com/entireio/entire-investigate/internal/cli"
)

var version = "dev"

func main() {
	if err := cli.Execute(version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
