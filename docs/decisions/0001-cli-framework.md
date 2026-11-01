# 0001: Use cobra as the CLI framework

## Context

`gh-receipts` will have several subcommands (`today`, `criteria`, `health`, `verify`) that take positional arguments and flags, such as `gh receipts criteria 5 --json`.

## Decision

Use `github.com/spf13/cobra`.

## Alternatives considered

- **Standard `flag` package:** no dependency, but parsing stops at the first positional argument, so `--json` in the example above would be ignored. We would also have to write subcommand dispatch and help output ourselves.

## Consequences

- Flags work before or after positional arguments, and `--version`, subcommands and help come for free.
- We take on `cobra` and `pflag` as dependencies.
