package envvar

import (
	"encoding/json"
	"testing"
)

func TestNormalize(t *testing.T) {
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
			got, err := Normalize(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Normalize() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValueAsString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		in     any
		want   string
		wantOK bool
	}{
		{name: "string", in: "hello", want: "hello", wantOK: true},
		{name: "empty string", in: "", want: "", wantOK: true},
		{name: "bool true", in: true, want: "true", wantOK: true},
		{name: "bool false", in: false, want: "false", wantOK: true},
		{name: "float", in: 3.14, want: "3.14", wantOK: true},
		{name: "json number", in: json.Number("42"), want: "42", wantOK: true},
		{name: "nil", in: nil, wantOK: false},
		{name: "map", in: map[string]string{"k": "v"}, wantOK: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ValueAsString(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("ValueAsString() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("ValueAsString() = %q, want %q", got, tt.want)
			}
		})
	}
}
