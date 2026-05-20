# Changelog

All notable changes to opvar are documented in this file. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-05-20

### Added

- Provider abstraction (`internal/provider`) so additional backends can be
  added without touching CLI or orchestration code.
- YAML config file at `~/.config/opvar/config.yaml` (override with
  `$OPVAR_CONFIG` or `$XDG_CONFIG_HOME`). Missing file falls back to defaults.
- `--provider NAME` flag to override the configured provider per invocation.
- GitHub Actions release pipeline (`.github/workflows/release.yml`) that
  cross-builds for darwin/linux on amd64/arm64 via goreleaser and publishes a
  Homebrew formula to `mrcat71/homebrew-tap` on `vX.Y.Z` tag push.
- Continuous integration workflow (`.github/workflows/ci.yml`) running
  `gofmt`, `go vet`, and `go test` on every push and pull request.
- Renovate configuration (`.github/renovate.json`) for automated dependency
  updates.

### Changed

- Repository layout switched to the standard Go `cmd/<binary>` +
  `internal/<package>` form.
- Module path renamed from `opvar` to `github.com/mrcat71/opvar` in
  preparation for public release.
- Minimum Go version bumped to 1.25.

### Removed

- Single-file `main.go` / `main_test.go`. Their contents now live in the
  appropriate `internal/...` packages with focused tests.

## [0.0.1] - 2026-02-11

### Added

- Initial release: list 1Password items by tag and emit shell `export`
  commands or JSON output.
