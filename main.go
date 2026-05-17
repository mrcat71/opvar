package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const commandTimeout = 30 * time.Second
const maxFetchWorkers = 8

type opRunner interface {
	RunJSON(ctx context.Context, args []string, out any) error
}

type execOpRunner struct{}

type itemSummary struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
}

type itemDetails struct {
	ID     string      `json:"id"`
	Title  string      `json:"title"`
	Fields []itemField `json:"fields"`
}

type itemField struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Type    string `json:"type"`
	Purpose string `json:"purpose"`
	Value   any    `json:"value"`
}

type envPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Item  string `json:"item"`
	Field string `json:"field,omitempty"`
}

var (
	version = "dev"
)

func main() {
	os.Exit(run(os.Args[1:], execOpRunner{}, os.Stdout, os.Stderr))
}

func run(args []string, runner opRunner, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("opvar", flag.ContinueOnError)
	fs.SetOutput(stderr)

	jsonOutput := fs.Bool("json", false, "output secrets as JSON instead of shell export commands")
	strict := fs.Bool("strict", false, "fail on first invalid item instead of skipping it")
	help := fs.Bool("help", false, "show help")
	shortVersion := fs.Bool("v", false, "show version")
	fullVersion := fs.Bool("version", false, "show version")

	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage:")
		fmt.Fprintln(stderr, "  opvar [--json] [--strict] <label>")
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
		fmt.Fprintln(stdout, versionLine())
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

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	pairs, warnings, err := collectEnvPairs(ctx, runner, label, *strict)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
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
		fmt.Fprintf(stdout, "export %s=%s\n", pair.Name, shellQuote(pair.Value))
	}

	return 0
}

func versionLine() string {
	return version
}

func collectEnvPairs(ctx context.Context, runner opRunner, label string, strict bool) ([]envPair, []string, error) {
	filtered, warnings, err := listItemsByLabel(ctx, runner, label)
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
	pairs := make([]envPair, 0, len(filtered))

	results := fetchItemDetailsParallel(ctx, runner, filtered)

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

func listItemsByLabel(ctx context.Context, runner opRunner, label string) ([]itemSummary, []string, error) {
	var items []itemSummary
	err := runner.RunJSON(ctx, []string{"item", "list", "--tags", label, "--format", "json"}, &items)
	if err == nil {
		return items, nil, nil
	}

	if !isUnknownFlagError(err) {
		return nil, nil, fmt.Errorf("failed to list items in 1Password: %w", err)
	}

	// Backward-compatible fallback for old op CLI versions.
	var allItems []itemSummary
	if fallbackErr := runner.RunJSON(ctx, []string{"item", "list", "--format", "json"}, &allItems); fallbackErr != nil {
		return nil, nil, fmt.Errorf("failed to list items in 1Password: %w", fallbackErr)
	}

	filtered := make([]itemSummary, 0, len(allItems))
	for _, item := range allItems {
		if hasLabel(item.Tags, label) {
			filtered = append(filtered, item)
		}
	}

	warnings := []string{"your op CLI does not support --tags; using slower fallback item listing"}
	return filtered, warnings, nil
}

type itemFetchResult struct {
	details itemDetails
	err     error
}

func fetchItemDetailsParallel(ctx context.Context, runner opRunner, items []itemSummary) []itemFetchResult {
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
				var details itemDetails
				err := runner.RunJSON(ctx, []string{"item", "get", items[idx].ID, "--format", "json"}, &details)
				results[idx] = itemFetchResult{
					details: details,
					err:     err,
				}
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

func buildItemEnvPairs(item itemSummary, details itemDetails) ([]envPair, []string, error) {
	pairs := make([]envPair, 0, len(details.Fields))
	warnings := make([]string, 0)

	for _, field := range details.Fields {
		if isNonSecretField(field) || isPrimaryCredentialField(field) {
			continue
		}

		value, ok := valueAsString(field.Value)
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

		name, err := normalizeEnvName(rawFieldName)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("item %q field %q skipped: %v", item.Title, rawFieldName, err))
			continue
		}

		pairs = append(pairs, envPair{
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
	name, err := normalizeEnvName(item.Title)
	if err != nil {
		return nil, warnings, err
	}

	value, err := extractSecretValue(details.Fields)
	if err != nil {
		return nil, warnings, err
	}

	return []envPair{
		{
			Name:  name,
			Value: value,
			Item:  item.Title,
		},
	}, warnings, nil
}

func (r execOpRunner) RunJSON(ctx context.Context, args []string, out any) error {
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

func hasLabel(tags []string, label string) bool {
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

func isUnknownFlagError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "unknown flag")
}

func normalizeEnvName(title string) (string, error) {
	raw := strings.TrimSpace(title)
	if raw == "" {
		return "", errors.New("empty item title")
	}

	var b strings.Builder
	underscore := false

	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			underscore = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
			underscore = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			underscore = false
		default:
			if !underscore {
				b.WriteByte('_')
				underscore = true
			}
		}
	}

	name := strings.Trim(b.String(), "_")
	if name == "" {
		return "", errors.New("item title has no usable characters for env name")
	}

	if name[0] >= '0' && name[0] <= '9' {
		name = "OPVAR_" + name
	}

	return name, nil
}

func extractSecretValue(fields []itemField) (string, error) {
	priorityChecks := []func(itemField) bool{
		func(field itemField) bool { return strings.EqualFold(field.Purpose, "PASSWORD") },
		func(field itemField) bool {
			return strings.EqualFold(field.ID, "password") || strings.EqualFold(field.Label, "password")
		},
		func(field itemField) bool { return strings.EqualFold(field.Type, "CONCEALED") },
		func(itemField) bool { return true },
	}

	for _, check := range priorityChecks {
		for _, field := range fields {
			if isNonSecretField(field) || isPrimaryCredentialField(field) {
				continue
			}
			if !check(field) {
				continue
			}
			value, ok := valueAsString(field.Value)
			if !ok || value == "" {
				continue
			}
			return value, nil
		}
	}

	return "", errors.New("no non-empty additional secret value field found")
}

func isNonSecretField(field itemField) bool {
	if strings.EqualFold(field.Purpose, "NOTES") {
		return true
	}
	if strings.EqualFold(field.ID, "notesPlain") {
		return true
	}
	return false
}

func isPrimaryCredentialField(field itemField) bool {
	if strings.EqualFold(field.Purpose, "USERNAME") || strings.EqualFold(field.Purpose, "PASSWORD") {
		return true
	}

	id := strings.TrimSpace(field.ID)
	if strings.EqualFold(id, "username") || strings.EqualFold(id, "password") {
		return true
	}

	return false
}

func valueAsString(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case bool:
		if v {
			return "true", true
		}
		return "false", true
	case float64:
		return fmt.Sprintf("%v", v), true
	case json.Number:
		return v.String(), true
	default:
		return "", false
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
