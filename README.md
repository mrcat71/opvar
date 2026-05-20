# opvar

`opvar` is a small CLI that loads secrets from a password manager by tag (label) and prints shell `export` commands or JSON.

Today it ships with a single backend, **1Password**, but the provider layer is designed so additional backends (e.g. Bitwarden) can plug in without touching the rest of the codebase. The active provider can be selected per machine via `~/.config/opvar/config.yaml` or per invocation via `--provider`.

## Install

### Homebrew (recommended)

```bash
brew tap mrcat71/tap
brew install mrcat71/tap/opvar
```

### From source

```bash
go install github.com/mrcat71/opvar/cmd/opvar@latest
```

### Local build

```bash
make build         # produces ./dist/opvar
make install BINDIR="$HOME/.local/bin"
```

## Requirements

- 1Password CLI [`op`](https://developer.1password.com/docs/cli/) on `PATH`
- An active `op` session (`op signin`)

## Usage

```bash
opvar <label>
```

Apply the exports to the current shell in one step:

```bash
eval "$(opvar okira-infra)"
```

If you want a shortcut in `~/.zshrc`:

```bash
opvar-use() { eval "$(command opvar "$@")"; }
```

Avoid shadowing the `opvar` command itself with an alias, otherwise the plain `opvar --help` no longer works.

## Flags

- `--json` print results as JSON instead of `export` lines
- `--strict` fail on the first invalid item (default: skip and warn)
- `--provider NAME` override the configured provider (e.g. `1password`)
- `--help` show usage
- `--v` / `--version` show version

## How it works

1. Lists items matching the label via `op item list --tags <label> --format json` (server-side filtering, with a slower client-side fallback for old `op` versions that don't support `--tags`).
2. Fetches each item's details in parallel via `op item get <id> --format json`.
3. For each item:
   - Uses each field's `label` (or `id` if label is empty) as the env var name.
   - Skips notes (`notesPlain` / `NOTES`) and the primary username/password credential.
4. If an item has no named exportable fields, falls back to:
   - Var name = item title
   - Value = highest-priority secret field (PASSWORD purpose, then `password` id/label, then any CONCEALED type).
5. Prints `export NAME='value'` lines to stdout, one per resolved pair.

Diagnostics (skipped items, fallback notices, duplicate variable names) are written to stderr as `warning: ...` lines.

## Configuration

Optional YAML config at `~/.config/opvar/config.yaml` (override path with `$OPVAR_CONFIG`, override base dir with `$XDG_CONFIG_HOME`):

```yaml
# Default provider; only "1password" is supported today.
provider: 1password

providers:
  1password:
    # reserved for future per-provider tuning (account, vault, ...)
```

If the file is missing the defaults (`provider: 1password`) apply, so existing users don't need to create anything.

The `--provider` CLI flag always wins over the config file.

## Version

```bash
opvar --v
opvar --version
```

Both print the short semantic version (e.g. `0.1.0`). When installed via Homebrew or `go install`, the version is baked in at build time. For local Makefile builds, the version comes from the `VERSION` file in the repo root.

## Releasing

Tagged releases are built by GitHub Actions + [goreleaser](https://goreleaser.com/). Pushing a `vX.Y.Z` tag triggers:

1. Cross-builds for `darwin`/`linux` × `amd64`/`arm64`.
2. A GitHub Release with archives and `checksums.txt`.
3. An updated Formula push to [`mrcat71/homebrew-tap`](https://github.com/mrcat71/homebrew-tap).

See `.goreleaser.yaml` and `.github/workflows/release.yml`.

## License

[Apache-2.0](LICENSE)
