# opvar

`opvar` is a macOS CLI utility that loads secrets from 1Password by tag (label) and prints shell `export` commands.

## Requirements

- [1Password CLI](https://developer.1password.com/docs/cli/) installed
- Active `op` session (`op signin`)

## How It Works

1. Runs `op item list --tags <label> --format json` (server-side filtering)
2. Loads matched item details in parallel via `op item get <id> --format json`
3. For each matched item:
   - Uses each field `label` (or `id` if label is empty) as the environment variable name
   - Uses the field value as the environment variable value
   - Skips notes fields (`notesPlain` / `NOTES`)
4. If an item has no named exportable fields:
   - Fallback variable name = item `title`
   - Fallback value = first suitable secret field
5. Prints `export ...` lines to stdout

## Usage

```bash
opvar <label>
```

Example:

```bash
eval "$(opvar <label>)"
```

This applies exported values to your current shell session.

## Apply Immediately In Shell

`opvar <label>` prints `export ...` lines.  
To apply them in the current shell, use:

```bash
eval "$(opvar <label>)"
```

If you want a shortcut in `~/.zshrc`, use a function:

```bash
opvar-use() { eval "$(command opvar "$@")"; }
```

Avoid replacing the `opvar` command name itself with an alias, otherwise you lose normal CLI usage.

## Flags

- `--json` output key/value pairs as JSON
- `--strict` fail on the first invalid item (default: skip invalid items and print warnings)
- `--help` show usage
- `--v` show version
- `--version` show version

## Build

```bash
make build
```

Build output: `dist/opvar`

## Install

Common Go project options are `Makefile` or `go install`.

Recommended (`Makefile`, explicit target path + correct permissions):

```bash
make install BINDIR="$HOME/.local/bin"
```

System path install:

```bash
sudo make install BINDIR="/usr/local/bin"
```

The install step uses `install -m 0755`, so the binary is installed as executable.

Alternative without `Makefile`:

```bash
GOBIN="$HOME/.local/bin" go install .
```

## Version

```bash
opvar --v
opvar --version
```

Both commands print the short semantic version (for example `0.0.1`).

Version source is the local `VERSION` file in the project root.  
Update it and rebuild:

```bash
echo "0.0.2" > VERSION
make build
```
