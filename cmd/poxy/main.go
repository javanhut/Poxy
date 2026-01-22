// Poxy - Universal Package Manager
package main

import (
	"fmt"
	"os"

	"poxy/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
