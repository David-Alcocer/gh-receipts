### Context

To recommend what's next, an agent needs more than the list of issues and pull requests in `next`. In a team using gh-receipts for a full working session, the agent had to call `gh` directly for:

- each issue's labels (the team ranks work with `priority:*` labels) and milestone;
- which issues block it, and whether those are still open;
- whether each open pull request can be merged (behind the base branch, blocked by rules) and which issues it closes.

GitHub has native fields for all of this: `labels`, `milestone`, `blockedBy` (issue dependencies, with each blocking issue's state, also across repositories), `mergeStateStatus` and `closingIssuesReferences`. gh-receipts should report them as facts and leave the ranking to each team's own rules (for example in their CLAUDE.md), since teams name priorities differently.

### Acceptance criteria

- [ ] `next --json` includes, for each assigned issue, its labels, its milestone (title and URL, or `null`) and the issues blocking it, each with number, repository, state and URL
- [ ] `next --json` includes a `blocked` field per issue: `true` when at least one blocking issue is open
- [ ] `next --json` includes, for each open pull request of the viewer, its merge state as GitHub reports it and the issues it closes, each with number, state and URL
- [ ] The `next` text report and `context` show labels, "blocked by #N" and the merge state briefly on each line, within the existing section limits
- [ ] Labels are reported as they are, without assuming a naming scheme such as `priority:*`
- [ ] Dependencies written only as text in the issue body (such as "Depends on: #12") are not parsed; the docs say how to turn them into native GitHub relations
- [ ] The change is additive, so `schemaVersion` stays `1`; `docs/next-json.md` documents the new fields
- [ ] Fetching and rendering are tested with recorded fixtures, without network

### Notes and hints

- Fields checked against the live API: `Issue.labels`, `Issue.milestone`, `Issue.blockedBy(first:)` with `state` and `repository`, `Issue.issueDependenciesSummary`, `PullRequest.mergeStateStatus`, `PullRequest.closingIssuesReferences`.
- `mergeStateStatus` can be `UNKNOWN` while GitHub computes it. Report it as it comes and document that.
- Measure the query on a busy repository before and after, as #29 did.

### Out of scope

- Ranking issues or choosing the next one: that's the agent's job with the team's rules.
- Parsing dependency text from issue bodies.
- Issues assigned to other people (a team view is a separate issue).
