// Package onepassword implements the Provider interface against the
// 1Password CLI (`op`). It shells out to `op` for listing items by tag and
// fetching individual item details.
package onepassword

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mrcat71/opvar/internal/secret"
)

// Name is the canonical provider identifier used in config files and the
// --provider flag.
const Name = "1password"

// Runner is the seam used to swap the real `op` shellout for a fake in tests.
type Runner interface {
	RunJSON(ctx context.Context, args []string, out any) error
}

// Provider talks to the 1Password CLI via a Runner.
type Provider struct {
	Runner Runner
}

// New returns a Provider that shells out to the real `op` binary.
func New() *Provider {
	return &Provider{Runner: execRunner{}}
}

// Name reports the provider's canonical identifier.
func (p *Provider) Name() string { return Name }

// ListItemsByLabel returns items tagged with the given label. It prefers
// `op item list --tags`; if the local `op` does not understand --tags it
// falls back to listing all items and filtering client-side. The fallback
// path emits a warning so the user can upgrade their CLI.
func (p *Provider) ListItemsByLabel(ctx context.Context, label string) ([]secret.Item, []string, error) {
	var items []secret.Item
	err := p.Runner.RunJSON(ctx, []string{"item", "list", "--tags", label, "--format", "json"}, &items)
	if err == nil {
		return items, nil, nil
	}

	if !isUnknownFlagError(err) {
		return nil, nil, fmt.Errorf("failed to list items in 1Password: %w", err)
	}

	var allItems []secret.Item
	if fallbackErr := p.Runner.RunJSON(ctx, []string{"item", "list", "--format", "json"}, &allItems); fallbackErr != nil {
		return nil, nil, fmt.Errorf("failed to list items in 1Password: %w", fallbackErr)
	}

	filtered := make([]secret.Item, 0, len(allItems))
	for _, item := range allItems {
		if HasLabel(item.Tags, label) {
			filtered = append(filtered, item)
		}
	}

	warnings := []string{"your op CLI does not support --tags; using slower fallback item listing"}
	return filtered, warnings, nil
}

// FetchItemDetails returns the full field set for one item.
func (p *Provider) FetchItemDetails(ctx context.Context, id string) (secret.ItemDetails, error) {
	var details secret.ItemDetails
	err := p.Runner.RunJSON(ctx, []string{"item", "get", id, "--format", "json"}, &details)
	return details, err
}

// HasLabel reports whether the given tag list contains the target label or a
// nested tag of the form "<label>/<sub>". Matching is case-insensitive and
// ignores surrounding whitespace.
func HasLabel(tags []string, label string) bool {
	target := strings.ToLower(strings.TrimSpace(label))
	if target == "" {
		return false
	}

	for _, tag := range tags {
		current := strings.ToLower(strings.TrimSpace(tag))
		if current == target || strings.HasPrefix(current, target+"/") {
			return true
		}
	}

	return false
}

// IsExecNotFound reports whether err is the "op binary not in PATH" error,
// which the CLI surfaces with a dedicated message.
func IsExecNotFound(err error) bool {
	return errors.Is(err, exec.ErrNotFound)
}

func isUnknownFlagError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "unknown flag")
}

// execRunner is the production Runner; it shells out to the real `op` binary.
type execRunner struct{}

func (execRunner) RunJSON(ctx context.Context, args []string, out any) error {
	cmd := exec.CommandContext(ctx, "op", args...)
	data, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr != "" {
				return fmt.Errorf("%w: %s", err, stderr)
			}
		}
		return err
	}

	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("invalid JSON from op: %w", err)
	}
	return nil
}
