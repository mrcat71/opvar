// Package provider defines the Provider interface every secret backend
// (1Password today, more later) implements.
package provider

import (
	"context"

	"github.com/mrcat71/opvar/internal/secret"
)

// Provider abstracts a password manager backend.
//
// ListItemsByLabel returns items matching a user-supplied label, plus any
// non-fatal warnings (e.g. fallback notices). FetchItemDetails returns the
// full field set for one item.
type Provider interface {
	Name() string
	ListItemsByLabel(ctx context.Context, label string) ([]secret.Item, []string, error)
	FetchItemDetails(ctx context.Context, id string) (secret.ItemDetails, error)
}
