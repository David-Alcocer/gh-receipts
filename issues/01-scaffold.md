### Context

Everything else depends on a working extension skeleton with CI and releases. Setting this up first means every later PR is checked automatically and `main` stays green.

### Acceptance criteria

- [ ] Repo is named `gh-receipts` (gh extensions must start with `gh-`) and was scaffolded with `gh extension create --precompiled=go` or an equivalent layout
- [ ] `gh receipts --version` prints the version embedded at build time
- [ ] Auth and API clients come from `github.com/cli/go-gh/v2` — no manual token handling
- [ ] CI workflow runs `go build`, `go vet` and `go test ./...` on every PR and on push to `main`
- [ ] Release workflow publishes precompiled binaries when a `v*` tag is pushed (using `cli/gh-extension-precompile`)
- [ ] Local install with `gh extension install .` is documented in the README
- [ ] Branch protection on `main` requires CI to pass before merging
- [ ] The CLI framework decision (standard `flag` package vs. `cobra`) is recorded in `docs/decisions/0001-cli-framework.md`

### Notes and hints

- `go-gh` docs: https://github.com/cli/go-gh
- `cli/gh-extension-precompile` builds with fixed `-ldflags="-s -w"`, so embedding the version needs its `build_script_override` input (or reading `runtime/debug.ReadBuildInfo`).
- Branch protection needs a public repo or a paid plan.
- This is a good issue to let the agent do most of the work, but review the plan first and make sure you understand every workflow file.

### Out of scope

- Any real command logic beyond `--version`.
