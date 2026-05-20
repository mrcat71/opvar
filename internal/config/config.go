// Package config loads opvar's YAML config from $OPVAR_CONFIG or
// $XDG_CONFIG_HOME/opvar/config.yaml (default ~/.config/opvar/config.yaml).
//
// A missing file is not an error; defaults still apply so zero-config users
// keep working.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Config is opvar's parsed configuration.
type Config struct {
	Provider  string                    `koanf:"provider"`
	Providers map[string]map[string]any `koanf:"providers"`
}

// DefaultProvider is used when no config file exists and no flag overrides.
const DefaultProvider = "1password"

// SupportedProviders lists every provider name opvar accepts in config or
// via --provider. New providers must be appended here.
var SupportedProviders = []string{"1password"}

// Load reads the config file resolved by Path. A missing file returns the
// default config and no error.
func Load() (Config, string, error) {
	path, err := Path()
	if err != nil {
		return Config{}, "", err
	}

	cfg := Config{Provider: DefaultProvider}

	if path == "" {
		return cfg, "", nil
	}

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return cfg, path, nil
	} else if err != nil {
		return Config{}, path, fmt.Errorf("stat config %s: %w", path, err)
	}

	k := koanf.New(".")
	if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
		return Config{}, path, fmt.Errorf("load config %s: %w", path, err)
	}

	if err := k.Unmarshal("", &cfg); err != nil {
		return Config{}, path, fmt.Errorf("parse config %s: %w", path, err)
	}

	if cfg.Provider == "" {
		cfg.Provider = DefaultProvider
	}

	return cfg, path, nil
}

// Path returns the config file path that Load will consult. Empty string
// means no candidate path could be derived (no $HOME, no $XDG_CONFIG_HOME,
// no $OPVAR_CONFIG).
func Path() (string, error) {
	if explicit := strings.TrimSpace(os.Getenv("OPVAR_CONFIG")); explicit != "" {
		return explicit, nil
	}

	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "opvar", "config.yaml"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil
	}
	return filepath.Join(home, ".config", "opvar", "config.yaml"), nil
}

// Validate checks that the resolved provider name is one opvar supports.
// It is separated from Load so callers can apply --provider overrides first.
func Validate(cfg Config) error {
	name := strings.TrimSpace(cfg.Provider)
	if name == "" {
		return errors.New("provider is empty")
	}

	for _, supported := range SupportedProviders {
		if name == supported {
			return nil
		}
	}

	supported := append([]string(nil), SupportedProviders...)
	sort.Strings(supported)
	return fmt.Errorf("unknown provider %q, supported: %s", name, strings.Join(supported, ", "))
}
