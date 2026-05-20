// Command opvar exports secrets from a configurable provider (1Password by
// default) as shell export lines or JSON.
package main

import (
	"os"

	"github.com/mrcat71/opvar/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
