# Security Policy

## Supported versions

Only the latest minor version receives security updates.

## Reporting a vulnerability

Please report security issues privately via GitHub Security Advisories:
<https://github.com/mrcat71/opvar/security/advisories/new>

I aim to acknowledge reports within 7 days and ship a fix within 30 days
for confirmed issues. Please do not file public issues for suspected
vulnerabilities.

## Security model

`opvar` is a thin wrapper that lifts secrets out of a password manager and
into a shell environment via `eval "$(opvar <label>)"`. It explicitly
trusts:

- The local `op` binary on `PATH`. If your `PATH` is writable by another
  user or process they can replace `op` with a fake that returns arbitrary
  values. Keep `op` in a directory only you can write to.
- The contents of the 1Password vault that the active `op` session can
  read. Anything in the matched items will end up in the shell as soon as
  the user runs `eval`.
- The user's shell that consumes the `export` lines. `opvar` produces
  POSIX-safe single-quoted strings, so values cannot inject shell code,
  but the shell still trusts the value once it is assigned.

## What opvar guards against

- **Shell injection in values.** All values are single-quoted and any
  embedded single quotes are escaped, so a malicious value cannot break
  out of the assignment.
- **Reserved env var hijack.** A vault item whose field label normalizes
  to a reserved shell or dynamic-loader variable (`PATH`, `LD_PRELOAD`,
  `LD_LIBRARY_PATH`, `DYLD_LIBRARY_PATH`, `DYLD_INSERT_LIBRARIES`,
  `SHELL`, `IFS`, `HOME`, `USER`, `PS1` through `PS4`,
  `PROMPT_COMMAND`, `BASH_ENV`, `ENV`) is dropped with a warning. Under
  `--strict` the run fails. This blocks shared-vault attacks where a
  teammate adds a malicious item to a vault you read.

## What opvar does not guard against

- **Compromised `op` binary on `PATH`.** Out of scope; verify your
  `PATH` and the `op` binary's location yourself.
- **Compromised 1Password account or vault.** If your vault is owned,
  so is your shell after `eval`.
- **stderr leakage.** Item titles and field names land on stderr as
  diagnostics. If you pipe stderr to a log file, vault metadata
  (names of secrets, not values) is captured. Values themselves never
  appear on stderr.
