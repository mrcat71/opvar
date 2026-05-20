package secret

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// fakeFetcher implements Fetcher with canned responses for tests.
type fakeFetcher struct {
	listItems    []Item
	listWarnings []string
	listErr      error

	details    map[string]ItemDetails
	detailErrs map[string]error
}

func (f *fakeFetcher) ListItemsByLabel(_ context.Context, _ string) ([]Item, []string, error) {
	if f.listErr != nil {
		return nil, f.listWarnings, f.listErr
	}
	return f.listItems, f.listWarnings, nil
}

func (f *fakeFetcher) FetchItemDetails(_ context.Context, id string) (ItemDetails, error) {
	if err := f.detailErrs[id]; err != nil {
		return ItemDetails{}, err
	}
	return f.details[id], nil
}

func mustUnmarshalDetails(t *testing.T, payload string) ItemDetails {
	t.Helper()
	var d ItemDetails
	if err := json.Unmarshal([]byte(payload), &d); err != nil {
		t.Fatalf("invalid test payload: %v", err)
	}
	return d
}

func TestCollect(t *testing.T) {
	t.Parallel()

	f := &fakeFetcher{
		listItems: []Item{{ID: "1", Title: "test-opvar", Tags: []string{"opvar-test"}}},
		details: map[string]ItemDetails{
			"1": mustUnmarshalDetails(t, `{
				"id":"1","title":"test-opvar",
				"fields":[
					{"id":"test1","label":"test1","type":"CONCEALED","value":"value-1"},
					{"id":"test2","label":"test2","type":"CONCEALED","value":"value-2"},
					{"id":"notesPlain","purpose":"NOTES","value":"description"}
				]
			}`),
		},
	}

	pairs, warnings, err := Collect(context.Background(), f, "opvar-test", false)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("Collect() warnings = %v, want none", warnings)
	}
	if len(pairs) != 2 {
		t.Fatalf("Collect() len = %d, want 2", len(pairs))
	}
	if pairs[0].Name != "test1" || pairs[0].Value != "value-1" {
		t.Fatalf("unexpected pair: %+v", pairs[0])
	}
	if pairs[1].Name != "test2" || pairs[1].Value != "value-2" {
		t.Fatalf("unexpected pair: %+v", pairs[1])
	}
}

func TestCollectSkipsPrimaryCredentials(t *testing.T) {
	t.Parallel()

	f := &fakeFetcher{
		listItems: []Item{{ID: "1", Title: "service-login", Tags: []string{"opvar-test"}}},
		details: map[string]ItemDetails{
			"1": mustUnmarshalDetails(t, `{
				"id":"1","title":"service-login",
				"fields":[
					{"id":"username","purpose":"USERNAME","value":"alice"},
					{"id":"password","purpose":"PASSWORD","type":"CONCEALED","value":"top-secret"},
					{"id":"api_token","label":"api_token","type":"CONCEALED","value":"token-1"}
				]
			}`),
		},
	}

	pairs, warnings, err := Collect(context.Background(), f, "opvar-test", false)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("Collect() warnings = %v, want none", warnings)
	}
	if len(pairs) != 1 {
		t.Fatalf("Collect() len = %d, want 1", len(pairs))
	}
	if pairs[0].Name != "api_token" || pairs[0].Value != "token-1" {
		t.Fatalf("unexpected pair: %+v", pairs[0])
	}
}

func TestCollectStrictMode(t *testing.T) {
	t.Parallel()

	f := &fakeFetcher{
		listItems:  []Item{{ID: "1", Title: "db-password", Tags: []string{"okira-infra"}}},
		detailErrs: map[string]error{"1": errors.New("boom")},
	}

	_, _, err := Collect(context.Background(), f, "okira-infra", true)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected strict mode error with boom, got %v", err)
	}
}

func TestCollectPropagatesListWarnings(t *testing.T) {
	t.Parallel()

	f := &fakeFetcher{
		listItems:    []Item{{ID: "1", Title: "test-opvar", Tags: []string{"opvar-test"}}},
		listWarnings: []string{"your op CLI does not support --tags; using slower fallback item listing"},
		details: map[string]ItemDetails{
			"1": mustUnmarshalDetails(t, `{
				"id":"1","title":"test-opvar",
				"fields":[{"id":"test1","label":"test1","type":"CONCEALED","value":"value-1"}]
			}`),
		},
	}

	pairs, warnings, err := Collect(context.Background(), f, "opvar-test", false)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("Collect() len = %d, want 1", len(pairs))
	}
	if len(warnings) != 1 || !strings.Contains(strings.ToLower(warnings[0]), "does not support --tags") {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
}

func TestCollectSkipsReservedEnvNames(t *testing.T) {
	t.Parallel()

	f := &fakeFetcher{
		listItems: []Item{{ID: "1", Title: "deploy-config", Tags: []string{"opvar-test"}}},
		details: map[string]ItemDetails{
			"1": mustUnmarshalDetails(t, `{
				"id":"1","title":"deploy-config",
				"fields":[
					{"id":"api_token","label":"api_token","type":"CONCEALED","value":"safe-value"},
					{"id":"PATH","label":"PATH","type":"CONCEALED","value":"/evil/bin"}
				]
			}`),
		},
	}

	pairs, warnings, err := Collect(context.Background(), f, "opvar-test", false)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(pairs) != 1 || pairs[0].Name != "api_token" || pairs[0].Value != "safe-value" {
		t.Fatalf("expected only api_token pair, got %+v", pairs)
	}

	foundReservedWarning := false
	for _, w := range warnings {
		if strings.Contains(w, "reserved env var") && strings.Contains(w, "PATH") {
			foundReservedWarning = true
			break
		}
	}
	if !foundReservedWarning {
		t.Fatalf("expected warning about reserved env var PATH, got %v", warnings)
	}
}

func TestCollectStrictFailsOnReserved(t *testing.T) {
	t.Parallel()

	f := &fakeFetcher{
		listItems: []Item{{ID: "1", Title: "deploy-config", Tags: []string{"opvar-test"}}},
		details: map[string]ItemDetails{
			"1": mustUnmarshalDetails(t, `{
				"id":"1","title":"deploy-config",
				"fields":[
					{"id":"PATH","label":"PATH","type":"CONCEALED","value":"/evil/bin"}
				]
			}`),
		},
	}

	_, _, err := Collect(context.Background(), f, "opvar-test", true)
	if err == nil {
		t.Fatal("expected strict mode error for reserved env var")
	}
	if !strings.Contains(err.Error(), "reserved env var") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestExtractSecretValueSkipsPrimaryCredentials(t *testing.T) {
	t.Parallel()

	fields := []Field{
		{ID: "username", Purpose: "USERNAME", Value: "alice"},
		{ID: "password", Purpose: "PASSWORD", Value: "top-secret"},
		{ID: "api_token", Type: "CONCEALED", Value: "custom-secret"},
	}

	got, err := ExtractSecretValue(fields)
	if err != nil {
		t.Fatalf("ExtractSecretValue() error = %v", err)
	}
	if got != "custom-secret" {
		t.Fatalf("ExtractSecretValue() = %q, want %q", got, "custom-secret")
	}
}
