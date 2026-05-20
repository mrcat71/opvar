package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	t.Setenv("OPVAR_CONFIG", filepath.Join(t.TempDir(), "does-not-exist.yaml"))

	cfg, path, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Provider != DefaultProvider {
		t.Fatalf("Provider = %q, want %q", cfg.Provider, DefaultProvider)
	}
	if path == "" {
		t.Fatal("expected path to be set even for missing files")
	}
}

func TestLoadReadsYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("provider: 1password\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("OPVAR_CONFIG", cfgPath)

	cfg, path, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Provider != "1password" {
		t.Fatalf("Provider = %q, want %q", cfg.Provider, "1password")
	}
	if path != cfgPath {
		t.Fatalf("path = %q, want %q", path, cfgPath)
	}
}

func TestLoadEmptyProviderFallsBackToDefault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("providers:\n  1password: {}\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("OPVAR_CONFIG", cfgPath)

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Provider != DefaultProvider {
		t.Fatalf("Provider = %q, want %q", cfg.Provider, DefaultProvider)
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	if err := Validate(Config{Provider: "1password"}); err != nil {
		t.Fatalf("Validate(1password) error = %v", err)
	}

	err := Validate(Config{Provider: "bitwarden"})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("unexpected error message: %v", err)
	}

	if err := Validate(Config{Provider: ""}); err == nil {
		t.Fatal("expected error for empty provider")
	}
}

func TestPathPrefersOpvarConfigEnv(t *testing.T) {
	t.Setenv("OPVAR_CONFIG", "/tmp/explicit.yaml")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")

	got, err := Path()
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	if got != "/tmp/explicit.yaml" {
		t.Fatalf("Path() = %q, want /tmp/explicit.yaml", got)
	}
}

func TestPathUsesXDG(t *testing.T) {
	t.Setenv("OPVAR_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")

	got, err := Path()
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	if got != "/tmp/xdg/opvar/config.yaml" {
		t.Fatalf("Path() = %q, want /tmp/xdg/opvar/config.yaml", got)
	}
}
