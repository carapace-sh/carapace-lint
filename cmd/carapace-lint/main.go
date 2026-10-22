// Command carapace-lint runs the checks as a standalone linter.
//
// The golangci-lint plugin allows configuring the checks, the standalone
// binary always runs all of them with default settings.
package main

import (
	"fmt"
	"os"

	"golang.org/x/tools/go/analysis/multichecker"

	carapacelint "github.com/carapace-sh/carapace-lint"
)

// version is set at build time.
var version = "develop"

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "-version" || os.Args[1] == "--version") {
		fmt.Println(version)

		return
	}

	analyzers, err := carapacelint.Analyzers()
	if err != nil {
		fmt.Fprintln(os.Stderr, "carapace-lint:", err)
		os.Exit(1)
	}

	multichecker.Main(analyzers...)
}
