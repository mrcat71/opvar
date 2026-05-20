package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	t.Parallel()

	value := "a'b"
	got := ShellQuote(value)
	want := "'a'\"'\"'b'"
	if got != want {
		t.Fatalf("ShellQuote() = %q, want %q", got, want)
	}
}

func TestRunVersionFlags(t *testing.T) {
	oldVersion := Version
	Version = "1.2.3"
	t.Cleanup(func() { Version = oldVersion })

	tests := [][]string{
		{"--v"},
		{"--version"},
	}

	for _, args := range tests {
		var stdout bytes.Buffer
		var stderr bytes.Buffer

		exitCode := Run(args, &stdout, &stderr)
		if exitCode != 0 {
			t.Fatalf("Run(%v) exitCode = %d, want 0", args, exitCode)
		}
		if strings.TrimSpace(stdout.String()) != "1.2.3" {
			t.Fatalf("Run(%v) stdout = %q, want version", args, stdout.String())
		}
		if stderr.Len() != 0 {
			t.Fatalf("Run(%v) stderr = %q, want empty", args, stderr.String())
		}
	}
}

func TestRunHelp(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"--help"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("Run(--help) exitCode = %d, want 0", exitCode)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("Run(--help) stderr = %q, expected usage output", stderr.String())
	}
}

func TestRunNoArgs(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("Run() exitCode = %d, want 2", exitCode)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("Run() stderr = %q, expected usage output", stderr.String())
	}
}

func TestRunRejectsUnknownProvider(t *testing.T) {
	// Use a clean config dir so we don't pick up the developer's real config.
	t.Setenv("OPVAR_CONFIG", filepath.Join(t.TempDir(), "missing.yaml"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"--provider", "bitwarden", "okira-infra"}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("Run() exitCode = %d, want 2", exitCode)
	}
	if !strings.Contains(stderr.String(), "unknown provider") {
		t.Fatalf("Run() stderr = %q, expected unknown provider error", stderr.String())
	}
}
