package secret

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/mrcat71/opvar/internal/envvar"
)

const maxFetchWorkers = 8

// Fetcher is the subset of provider.Provider that collect uses. Keeping it
// here avoids an import cycle between secret and provider.
type Fetcher interface {
	ListItemsByLabel(ctx context.Context, label string) ([]Item, []string, error)
	FetchItemDetails(ctx context.Context, id string) (ItemDetails, error)
}

// Collect returns the env-var pairs for every item matching label.
//
// In non-strict mode, items that fail to load or have no exportable fields
// are skipped and reported via the warnings slice. In strict mode the first
// such failure aborts.
func Collect(ctx context.Context, f Fetcher, label string, strict bool) ([]EnvPair, []string, error) {
	filtered, warnings, err := f.ListItemsByLabel(ctx, label)
	if err != nil {
		return nil, nil, err
	}

	if len(filtered) == 0 {
		return nil, nil, fmt.Errorf("no items found with label %q", label)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return strings.ToLower(filtered[i].Title) < strings.ToLower(filtered[j].Title)
	})

	seenNames := make(map[string]string, len(filtered))
	pairs := make([]EnvPair, 0, len(filtered))

	results := fetchDetailsParallel(ctx, f, filtered)

	for idx, item := range filtered {
		if results[idx].err != nil {
			if strict {
				return nil, warnings, fmt.Errorf("failed to load item %q: %w", item.Title, results[idx].err)
			}
			warnings = append(warnings, fmt.Sprintf("item %q skipped: %v", item.Title, results[idx].err))
			continue
		}

		itemPairs, itemWarnings, err := buildItemEnvPairs(item, results[idx].details)
		warnings = append(warnings, itemWarnings...)
		if err != nil {
			if strict {
				return nil, warnings, fmt.Errorf("item %q: %w", item.Title, err)
			}
			warnings = append(warnings, fmt.Sprintf("item %q skipped: %v", item.Title, err))
			continue
		}

		for _, pair := range itemPairs {
			if envvar.IsReserved(pair.Name) {
				msg := fmt.Sprintf("item %q maps to reserved env var %q (refusing to overwrite)", item.Title, pair.Name)
				if strict {
					return nil, warnings, errors.New(msg)
				}
				warnings = append(warnings, msg)
				continue
			}

			if existingSource, ok := seenNames[pair.Name]; ok {
				msg := fmt.Sprintf("item %q maps to duplicate variable %q (already used by %q)", item.Title, pair.Name, existingSource)
				if strict {
					return nil, warnings, errors.New(msg)
				}
				warnings = append(warnings, msg)
				continue
			}

			pairs = append(pairs, pair)
			sourceName := pair.Item
			if pair.Field != "" {
				sourceName = pair.Item + ":" + pair.Field
			}
			seenNames[pair.Name] = sourceName
		}
	}

	if len(pairs) == 0 {
		return nil, warnings, fmt.Errorf("no exportable secrets found for label %q", label)
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Name < pairs[j].Name
	})

	return pairs, warnings, nil
}

type itemFetchResult struct {
	details ItemDetails
	err     error
}

func fetchDetailsParallel(ctx context.Context, f Fetcher, items []Item) []itemFetchResult {
	results := make([]itemFetchResult, len(items))
	if len(items) == 0 {
		return results
	}

	workers := workerCount(len(items))
	jobs := make(chan int)
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for idx := range jobs {
				details, err := f.FetchItemDetails(ctx, items[idx].ID)
				results[idx] = itemFetchResult{details: details, err: err}
			}
		}()
	}

	for idx := range items {
		jobs <- idx
	}
	close(jobs)
	wg.Wait()

	return results
}

func workerCount(totalItems int) int {
	if totalItems <= 0 {
		return 1
	}

	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	if workers > maxFetchWorkers {
		workers = maxFetchWorkers
	}
	if workers > totalItems {
		workers = totalItems
	}
	return workers
}

func buildItemEnvPairs(item Item, details ItemDetails) ([]EnvPair, []string, error) {
	pairs := make([]EnvPair, 0, len(details.Fields))
	warnings := make([]string, 0)

	for _, field := range details.Fields {
		if isNonSecretField(field) || isPrimaryCredentialField(field) {
			continue
		}

		value, ok := envvar.ValueAsString(field.Value)
		if !ok || value == "" {
			continue
		}

		rawFieldName := strings.TrimSpace(field.Label)
		if rawFieldName == "" {
			rawFieldName = strings.TrimSpace(field.ID)
		}
		if rawFieldName == "" {
			warnings = append(warnings, fmt.Sprintf("item %q has a field with value but no name", item.Title))
			continue
		}

		name, err := envvar.Normalize(rawFieldName)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("item %q field %q skipped: %v", item.Title, rawFieldName, err))
			continue
		}

		pairs = append(pairs, EnvPair{
			Name:  name,
			Value: value,
			Item:  item.Title,
			Field: rawFieldName,
		})
	}

	if len(pairs) > 0 {
		return pairs, warnings, nil
	}

	// Fallback for entries where the secret value exists, but fields are unnamed.
	name, err := envvar.Normalize(item.Title)
	if err != nil {
		return nil, warnings, err
	}

	value, err := ExtractSecretValue(details.Fields)
	if err != nil {
		return nil, warnings, err
	}

	return []EnvPair{
		{
			Name:  name,
			Value: value,
			Item:  item.Title,
		},
	}, warnings, nil
}

// ExtractSecretValue scans fields in priority order and returns the first
// non-empty value suitable for use as the item's primary exported secret.
// It is exported so provider-specific tests can exercise the field-scoring
// rules directly.
func ExtractSecretValue(fields []Field) (string, error) {
	priorityChecks := []func(Field) bool{
		func(field Field) bool { return strings.EqualFold(field.Purpose, "PASSWORD") },
		func(field Field) bool {
			return strings.EqualFold(field.ID, "password") || strings.EqualFold(field.Label, "password")
		},
		func(field Field) bool { return strings.EqualFold(field.Type, "CONCEALED") },
		func(Field) bool { return true },
	}

	for _, check := range priorityChecks {
		for _, field := range fields {
			if isNonSecretField(field) || isPrimaryCredentialField(field) {
				continue
			}
			if !check(field) {
				continue
			}
			value, ok := envvar.ValueAsString(field.Value)
			if !ok || value == "" {
				continue
			}
			return value, nil
		}
	}

	return "", errors.New("no non-empty additional secret value field found")
}

func isNonSecretField(field Field) bool {
	if strings.EqualFold(field.Purpose, "NOTES") {
		return true
	}
	if strings.EqualFold(field.ID, "notesPlain") {
		return true
	}
	return false
}

func isPrimaryCredentialField(field Field) bool {
	if strings.EqualFold(field.Purpose, "USERNAME") || strings.EqualFold(field.Purpose, "PASSWORD") {
		return true
	}

	id := strings.TrimSpace(field.ID)
	if strings.EqualFold(id, "username") || strings.EqualFold(id, "password") {
		return true
	}

	return false
}
