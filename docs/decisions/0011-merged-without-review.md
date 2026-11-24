# 0011: Pull requests merged without review

## Context

Issue #26. The median time to first review only uses merged pull requests that got a review by someone other than the author or a bot. The report said "from 4 of 16 PRs", but the other 12 couldn't be opened from it. On first use in another team's repository, they were most of the merged work. The section title `PULL REQUESTS: N merged` was also a fact without a URL.

## Decision

- **A count of merged pull requests without review,** next to the time to first review, with the same rule for what counts as a review. Count plus reviewed equals merged.
- **"No data" when nothing was merged, and "none" when every pull request had a review,** so the line is never missing.
- **JSON lists the exact pull requests, by number and URL, without logins.** A GitHub search can't express "no review by someone else", so the text links to `review:none`. This said it could list fewer; checked against GitHub in #35, it lists more, and `docs/health-json.md` gives the cases.
- **The merged count moves from the section title to its own line with a link,** so every fact keeps its source. This changes the text format from 0009; JSON only gains a field, so `schemaVersion` stays `1`.
- **No threshold.** How much review a team wants depends on the team.

## Alternatives considered

- **Listing the pull requests in the text:** exact, but long, next to the stale list. The JSON has them.
- **Documenting section titles as exempt from the URL rule:** less change, but a fact without a source.

## Consequences

- With automatic code reviewers, the gap between the count and the search link grows, because GitHub counts bot reviews. The docs explain it.
- A script that parsed `PULL REQUESTS: N merged` from the text breaks. `--json` is the stable interface.
