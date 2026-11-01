# Roadmap

Each version should be usable on its own. Ship small, use it on this repo, then move on.

## v0.1 — "Where did we leave off?"

Goal: replace "hey agent, what's next?" with a verifiable report.

- Scaffold the extension, CI and releases (#1)
- Activity data layer: PRs, reviews, comments, CI runs since a timestamp (#2)
- `today` command with session memory and `--json` output (#3)
- Project board section in `today` (#4)

**Done when:** the team uses `gh receipts today` at the start of every work session for one week, and nobody needs to ask the agent "where did we leave off?".

## v0.2 — "Is our flow healthy?"

- Acceptance criteria convention + parser + `criteria` debug command (#5)
- `health` command with team-level flow signals (#6)

**Done when:** `health` is reviewed in a team meeting and leads to at least one concrete process change.

## v0.3 — "Did this PR really close its issue?"

- `verify <pr>`: for each acceptance criterion, collect deterministic evidence (files changed, tests added or modified, CI status, checked boxes, review comments) and classify it as *met / partial / no evidence*, with links.
- No AI yet. Measure how far deterministic evidence gets before adding a model.

## v0.4 — AI summaries with citations (optional)

- Optional `--summarize` flag that sends the already-fetched facts to a model and requires every sentence to cite a URL from those facts.
- Any sentence without a citation is dropped and reported.

## Later

- Interactive TUI (Bubble Tea) for `today` and the Project board.
- Configurable sections and filters in `.github/receipts.yml`.

## Build rules

1. The maintainer writes the README and acceptance criteria.
2. For AI-assisted issues: the agent proposes a plan, the maintainer reviews it before implementation.
3. If the author can't explain a PR line by line, it doesn't get merged.
4. Small PRs, `main` always green, every PR closes an issue.
5. Significant decisions are recorded in `docs/decisions/` (one short markdown file each).

## Non-goals

- **Not a code reviewer.** It reports evidence; humans judge quality.
- **Not a replacement for talking to your team.** It should make conversations more specific, not unnecessary.
- **No per-person rankings or productivity scores.** Health signals are team-level only.
