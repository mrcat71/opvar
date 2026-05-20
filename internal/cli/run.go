// Package cli wires flag parsing, config loading, and provider selection
// into the run() entry point invoked by cmd/opvar.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mrcat71/opvar/internal/config"
	"github.com/mrcat71/opvar/internal/provider"
	"github.com/mrcat71/opvar/internal/provider/onepassword"
	"github.com/mrcat71/opvar/internal/secret"
)

const commandTimeout = 30 * time.Second

// Version is set at link time via -ldflags "-X github.com/mrcat71/opvar/internal/cli.Version=..."
var Version = "dev"

// Run is the CLI entry point. It returns a process exit code so cmd/opvar
// stays a trivial wrapper around os.Exit.
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("opvar", flag.ContinueOnError)
	fs.SetOutput(stderr)

	jsonOutput := fs.Bool("json", false, "output secrets as JSON instead of shell export commands")
	strict := fs.Bool("strict", false, "fail on first invalid item instead of skipping it")
	providerFlag := fs.String("provider", "", "override provider from config (e.g. 1password)")
	help := fs.Bool("help", false, "show help")
	shortVersion := fs.Bool("v", false, "show version")
	fullVersion := fs.Bool("version", false, "show version")

	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage:")
		fmt.Fprintln(stderr, "  opvar [--json] [--strict] [--provider NAME] <label>")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Examples:")
		fmt.Fprintln(stderr, "  eval \"$(opvar okira-infra)\"")
		fmt.Fprintln(stderr, "  opvar --json okira-infra")
		fmt.Fprintln(stderr, "  opvar --version")
	}

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *shortVersion || *fullVersion {
		fmt.Fprintln(stdout, Version)
		return 0
	}

	if *help {
		fs.Usage()
		return 0
	}

	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}

	label := strings.TrimSpace(fs.Arg(0))
	if label == "" {
		fmt.Fprintln(stderr, "label must not be empty")
		return 2
	}

	cfg, _, err := config.Load()
	if err != nil {
		fmt.Fprintf(stderr, "opvar error: %v\n", err)
		return 1
	}
	if override := strings.TrimSpace(*providerFlag); override != "" {
		cfg.Provider = override
	}
	if err := config.Validate(cfg); err != nil {
		fmt.Fprintf(stderr, "opvar error: %v\n", err)
		return 2
	}

	prov, err := buildProvider(cfg.Provider)
	if err != nil {
		fmt.Fprintf(stderr, "opvar error: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	pairs, warnings, err := secret.Collect(ctx, prov, label, *strict)
	if err != nil {
		if onepassword.IsExecNotFound(err) {
			fmt.Fprintln(stderr, "1Password CLI (op) was not found in PATH")
			return 1
		}
		fmt.Fprintf(stderr, "opvar error: %v\n", err)
		return 1
	}

	for _, warning := range warnings {
		fmt.Fprintf(stderr, "warning: %s\n", warning)
	}

	if *jsonOutput {
		payload, err := json.MarshalIndent(pairs, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "failed to encode JSON: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(payload))
		return 0
	}

	for _, pair := range pairs {
		fmt.Fprintf(stdout, "export %s=%s\n", pair.Name, ShellQuote(pair.Value))
	}

	return 0
}

func buildProvider(name string) (provider.Provider, error) {
	switch name {
	case onepassword.Name:
		return onepassword.New(), nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", name)
	}
}
