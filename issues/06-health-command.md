### Context

There's no magic formula for "is our workflow healthy?", but a few signals inspired by DORA metrics show when the flow is getting stuck. These are team-level signals, calculated directly from the API — no AI involved.

### Acceptance criteria

- [ ] `gh receipts health` reports, for the last 14 days (configurable with `--days`):
  - [ ] median PR size (additions + deletions)
  - [ ] median time from PR opened to first review (excluding the author and bots)
  - [ ] median time from PR opened to merged
  - [ ] open PRs with no activity for more than 2 days, with links
  - [ ] issues opened vs. closed per week
- [ ] Each metric shows how many PRs or issues it was computed from
- [ ] Thresholds are configurable in `.github/receipts.yml`; defaults and the reasoning behind them are documented
- [ ] Only team-level aggregates are shown — no per-person breakdown or ranking
- [ ] Metric calculations are pure functions, unit tested with fixtures
- [ ] `--json` output is supported

### Notes and hints

- Trunk-based development favors small PRs and short-lived branches; use that to justify default thresholds.
- Depends on: Activity data layer (it may need a variant that fetches a time window instead of "since").

### Out of scope

- Deployment frequency and change failure rate (need deployment data; consider later).
