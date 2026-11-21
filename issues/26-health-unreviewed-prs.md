### Context

`health` computes the median time to first review only from merged pull requests that got a review by someone other than the author or a bot. It says how many ("from 4 of 16 PRs"), but the pull requests merged without any review can't be opened from the report. In a real team sample, 12 of 16 merged pull requests had no review, which is probably the most important signal in that report.

Also, the section title `PULL REQUESTS: N merged` reports a fact without a URL. The metric lines below it have one, but the title doesn't.

### Acceptance criteria

- [ ] `health` text shows how many pull requests were merged without a review by someone other than the author or a bot, with a link to list them
- [ ] When every merged pull request had a review, it says so instead of leaving the line out
- [ ] `--json` includes that count; the change is additive, so `schemaVersion` stays `1`
- [ ] The `N merged` count has a URL in the text output, or the docs state why section titles don't
- [ ] `docs/health-json.md` explains the new count and any difference between the count and what its link lists
- [ ] Metric tests and golden files cover it, without network

### Notes and hints

- The count must come from the data `health` already computed, so it agrees with "from X of N PRs".
- GitHub search has `review:none`, but it may count reviews that `health` ignores (by the author or by bots), so the link could list fewer pull requests than the count. The JSON already handles a similar gap: the `mergedPullRequests` URL includes bots and the count doesn't. Document it the same way.
- `internal/report/health.go`, `healthSections`: the section title is built with `fmt.Sprintf("PULL REQUESTS: %d merged", ...)`.

### Out of scope

- Naming who merged without review. `health` stays team-level, with no logins.
- Changing how the median time to first review is computed.
