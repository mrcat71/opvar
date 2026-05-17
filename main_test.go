package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNormalizeEnvName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "simple", input: "db_password", want: "db_password"},
		{name: "spaces and dashes", input: "db password-prod", want: "db_password_prod"},
		{name: "starts with digit", input: "123 token", want: "OPVAR_123_token"},
		{name: "only symbols", input: "---", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeEnvName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("normalizeEnvName() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeEnvName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHasLabel(t *testing.T) {
	t.Parallel()

	if !hasLabel([]string{"okira-infra"}, "okira-infra") {
		t.Fatal("expected exact match")
	}
	if !hasLabel([]string{"okira-infra/prod"}, "okira-infra") {
		t.Fatal("expected nested tag match")
	}
	if hasLabel([]string{"another"}, "okira-infra") {
		t.Fatal("did not expect match")
	}
}

func TestExtractSecretValueSkipsPrimaryCredentials(t *testing.T) {
	t.Parallel()

	fields := []itemField{
		{ID: "username", Purpose: "USERNAME", Value: "alice"},
		{ID: "password", Purpose: "PASSWORD", Value: "top-secret"},
		{ID: "api_token", Type: "CONCEALED", Value: "custom-secret"},
	}

	got, err := extractSecretValue(fields)
	if err != nil {
		t.Fatalf("extractSecretValue() error = %v", err)
	}
	if got != "custom-secret" {
		t.Fatalf("extractSecretValue() = %q, want %q", got, "custom-secret")
	}
}

func TestShellQuote(t *testing.T) {
	t.Parallel()

	value := "a'b"
	got := shellQuote(value)
	want := "'a'\"'\"'b'"
	if got != want {
		t.Fatalf("shellQuote() = %q, want %q", got, want)
	}
}

func TestRunVersionFlags(t *testing.T) {
	oldVersion := version
	version = "1.2.3"
	t.Cleanup(func() {
		version = oldVersion
	})

	tests := [][]string{
		{"--v"},
		{"--version"},
	}

	for _, args := range tests {
		var stdout bytes.Buffer
		var stderr bytes.Buffer

		exitCode := run(args, fakeRunner{}, &stdout, &stderr)
		if exitCode != 0 {
			t.Fatalf("run(%v) exitCode = %d, want 0", args, exitCode)
		}
		if strings.TrimSpace(stdout.String()) != "1.2.3" {
			t.Fatalf("run(%v) stdout = %q, want version", args, stdout.String())
		}
		if stderr.Len() != 0 {
			t.Fatalf("run(%v) stderr = %q, want empty", args, stderr.String())
		}
	}
}

func TestRunHelp(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"--help"}, fakeRunner{}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("run(--help) exitCode = %d, want 0", exitCode)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("run(--help) stderr = %q, expected usage output", stderr.String())
	}
}

func TestCollectEnvPairs(t *testing.T) {
	t.Parallel()

	runner := fakeRunner{
		payloads: map[string]string{
			argsKey("item", "list", "--tags", "opvar-test", "--format", "json"): `[
				{"id":"1","title":"test-opvar","tags":["opvar-test"]}
			]`,
			argsKey("item", "get", "1", "--format", "json"): `{
				"id":"1",
				"title":"test-opvar",
				"fields":[
					{"id":"test1","label":"test1","type":"CONCEALED","value":"value-1"},
					{"id":"test2","label":"test2","type":"CONCEALED","value":"value-2"},
					{"id":"notesPlain","purpose":"NOTES","value":"description"}
				]
			}`,
		},
	}

	pairs, warnings, err := collectEnvPairs(context.Background(), runner, "opvar-test", false)
	if err != nil {
		t.Fatalf("collectEnvPairs() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("collectEnvPairs() warnings = %v, want none", warnings)
	}
	if len(pairs) != 2 {
		t.Fatalf("collectEnvPairs() len = %d, want 2", len(pairs))
	}
	if pairs[0].Name != "test1" || pairs[0].Value != "value-1" {
		t.Fatalf("unexpected pair: %+v", pairs[0])
	}
	if pairs[1].Name != "test2" || pairs[1].Value != "value-2" {
		t.Fatalf("unexpected pair: %+v", pairs[1])
	}
}

func TestCollectEnvPairsSkipsPrimaryCredentials(t *testing.T) {
	t.Parallel()

	runner := fakeRunner{
		payloads: map[string]string{
			argsKey("item", "list", "--tags", "opvar-test", "--format", "json"): `[
				{"id":"1","title":"service-login","tags":["opvar-test"]}
			]`,
			argsKey("item", "get", "1", "--format", "json"): `{
				"id":"1",
				"title":"service-login",
				"fields":[
					{"id":"username","purpose":"USERNAME","value":"alice"},
					{"id":"password","purpose":"PASSWORD","type":"CONCEALED","value":"top-secret"},
					{"id":"api_token","label":"api_token","type":"CONCEALED","value":"token-1"}
				]
			}`,
		},
	}

	pairs, warnings, err := collectEnvPairs(context.Background(), runner, "opvar-test", false)
	if err != nil {
		t.Fatalf("collectEnvPairs() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("collectEnvPairs() warnings = %v, want none", warnings)
	}
	if len(pairs) != 1 {
		t.Fatalf("collectEnvPairs() len = %d, want 1", len(pairs))
	}
	if pairs[0].Name != "api_token" || pairs[0].Value != "token-1" {
		t.Fatalf("unexpected pair: %+v", pairs[0])
	}
}

func TestCollectEnvPairsStrictMode(t *testing.T) {
	t.Parallel()

	runner := fakeRunner{
		payloads: map[string]string{
			argsKey("item", "list", "--tags", "okira-infra", "--format", "json"): `[
				{"id":"1","title":"db-password","tags":["okira-infra"]}
			]`,
		},
		errs: map[string]error{
			argsKey("item", "get", "1", "--format", "json"): errors.New("boom"),
		},
	}

	_, _, err := collectEnvPairs(context.Background(), runner, "okira-infra", true)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected strict mode error with boom, got %v", err)
	}
}

func TestCollectEnvPairsUnknownTagsFallback(t *testing.T) {
	t.Parallel()

	runner := fakeRunner{
		payloads: map[string]string{
			argsKey("item", "list", "--format", "json"): `[
				{"id":"1","title":"test-opvar","tags":["opvar-test"]},
				{"id":"2","title":"ignored","tags":["other"]}
			]`,
			argsKey("item", "get", "1", "--format", "json"): `{
				"id":"1",
				"title":"test-opvar",
				"fields":[{"id":"test1","label":"test1","type":"CONCEALED","value":"value-1"}]
			}`,
		},
		errs: map[string]error{
			argsKey("item", "list", "--tags", "opvar-test", "--format", "json"): errors.New("unknown flag: --tags"),
		},
	}

	pairs, warnings, err := collectEnvPairs(context.Background(), runner, "opvar-test", false)
	if err != nil {
		t.Fatalf("collectEnvPairs() error = %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("collectEnvPairs() len = %d, want 1", len(pairs))
	}
	if pairs[0].Name != "test1" || pairs[0].Value != "value-1" {
		t.Fatalf("unexpected pair: %+v", pairs[0])
	}
	if len(warnings) != 1 || !strings.Contains(strings.ToLower(warnings[0]), "does not support --tags") {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
}

type fakeRunner struct {
	payloads map[string]string
	errs     map[string]error
}

func (f fakeRunner) RunJSON(_ context.Context, args []string, out any) error {
	key := argsKey(args...)
	if err := f.errs[key]; err != nil {
		return err
	}

	payload, ok := f.payloads[key]
	if !ok {
		return errors.New("unexpected args: " + strings.Join(args, " "))
	}

	return json.Unmarshal([]byte(payload), out)
}

func argsKey(args ...string) string {
	return strings.Join(args, "\x00")
}
