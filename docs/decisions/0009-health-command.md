# 0009: Health signals

## Context

Issue #6 asks for team-level flow signals: PR size, time to first review, time to merge, stale PRs and issues per week. Numbers like these are easy to misread: a zero that means "no data", a median from two PRs, or a list that turns into a ranking.

## Decision

- **Fetch and compute are separate.** `health.Fetch` reads facts; `health.Compute` is a pure function tested without the network.
- **Medians come from PRs merged in the window,** so each one is a finished piece of work. Each metric carries its count, and a missing value is `no data` (`null`), never `0`.
- **No logins in the output,** not even in JSON. Bot PRs, bot issues and bot reviews are excluded, and so are reviews by the PR's author.
- **Calendar hours and 7-day blocks back from now.** Business hours depend on time zones and holidays the tool doesn't know, and calendar weeks would make the newest one partial. The weekend effect is documented instead.
- **Thresholds live in `.github/receipts.yml` under `health:`.** One `internal/config` package now reads that file for both `project` and `health`. An explicit `0` or a negative value is an error rather than a silent default.
- **Under an hour, text shows minutes.** Rounded hours turned 2-minute reviews into "0 h", which reads like missing data.

## Alternatives considered

- **Counting comments as a first review:** closer to "someone looked", but bots and the author comment too, and GitHub already has reviews for this.
- **All open PRs, then filtering:** simpler, but on cli/cli it reads every open PR. Searching `updated:<` reads only the stale ones.
- **Separate config readers per command:** less change to `project`, but two queries for the same file and two sets of validation rules.

## Consequences

- On a small team, the stale PR list links PRs whose authors are easy to tell apart. Docs say the list is for unblocking work.
- `updatedAt` changes with any update, including labels and bot comments, so a PR can look active without human work.
- Deployment frequency and change failure rate stay out until there's deployment data.
