package onepassword

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestHasLabel(t *testing.T) {
	t.Parallel()

	if !HasLabel([]string{"okira-infra"}, "okira-infra") {
		t.Fatal("expected exact match")
	}
	if !HasLabel([]string{"okira-infra/prod"}, "okira-infra") {
		t.Fatal("expected nested tag match")
	}
	if HasLabel([]string{"another"}, "okira-infra") {
		t.Fatal("did not expect match")
	}
	if HasLabel([]string{"okira-infra"}, "") {
		t.Fatal("empty label must not match anything")
	}
}

func TestListItemsByLabelHappyPath(t *testing.T) {
	t.Parallel()

	runner := FakeRunner{
		Payloads: map[string]string{
			ArgsKey("item", "list", "--tags", "opvar-test", "--format", "json"): `[
				{"id":"1","title":"a","tags":["opvar-test"]}
			]`,
		},
	}
	p := &Provider{Runner: runner}

	items, warnings, err := p.ListItemsByLabel(context.Background(), "opvar-test")
	if err != nil {
		t.Fatalf("ListItemsByLabel() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(items) != 1 || items[0].ID != "1" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestListItemsByLabelUnknownTagsFallback(t *testing.T) {
	t.Parallel()

	runner := FakeRunner{
		Payloads: map[string]string{
			ArgsKey("item", "list", "--format", "json"): `[
				{"id":"1","title":"a","tags":["opvar-test"]},
				{"id":"2","title":"b","tags":["other"]}
			]`,
		},
		Errs: map[string]error{
			ArgsKey("item", "list", "--tags", "opvar-test", "--format", "json"): errors.New("unknown flag: --tags"),
		},
	}
	p := &Provider{Runner: runner}

	items, warnings, err := p.ListItemsByLabel(context.Background(), "opvar-test")
	if err != nil {
		t.Fatalf("ListItemsByLabel() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != "1" {
		t.Fatalf("expected only the matching item, got %+v", items)
	}
	if len(warnings) != 1 || !strings.Contains(strings.ToLower(warnings[0]), "does not support --tags") {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
}

func TestFetchItemDetails(t *testing.T) {
	t.Parallel()

	runner := FakeRunner{
		Payloads: map[string]string{
			ArgsKey("item", "get", "1", "--format", "json"): `{
				"id":"1","title":"a","fields":[{"id":"f","label":"f","type":"CONCEALED","value":"v"}]
			}`,
		},
	}
	p := &Provider{Runner: runner}

	details, err := p.FetchItemDetails(context.Background(), "1")
	if err != nil {
		t.Fatalf("FetchItemDetails() error = %v", err)
	}
	if details.ID != "1" || len(details.Fields) != 1 || details.Fields[0].Value != "v" {
		t.Fatalf("unexpected details: %+v", details)
	}
}

// FakeRunner is exported for reuse by other packages' tests (e.g. internal/secret).
type FakeRunner struct {
	Payloads map[string]string
	Errs     map[string]error
}

func (f FakeRunner) RunJSON(_ context.Context, args []string, out any) error {
	key := ArgsKey(args...)
	if err := f.Errs[key]; err != nil {
		return err
	}

	payload, ok := f.Payloads[key]
	if !ok {
		return errors.New("unexpected args: " + strings.Join(args, " "))
	}

	return json.Unmarshal([]byte(payload), out)
}

// Compile-time assertion that FakeRunner satisfies Runner and FakeRunner-built
// Provider can be used wherever a secret-fetching backend is expected.
var _ Runner = FakeRunner{}

// ArgsKey is the canonical map key for FakeRunner; concatenates args with NUL.
func ArgsKey(args ...string) string {
	return strings.Join(args, "\x00")
}
