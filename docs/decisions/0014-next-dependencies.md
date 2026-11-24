# 0014: Priorities, dependencies and merge state in `next`

## Context

Issue #33. In a team's working session, the agent called `gh` directly to decide what was next: each issue's priority label, what blocks it and whether those blockers are still open, and whether each pull request could be merged and which issues it closes. Teams name priorities differently (`priority:high`, `P1`, `priority-3` on cli/cli) and write dependencies differently.

## Decision

- **Report GitHub's native fields as facts:** labels, milestone, `blockedBy` with each blocker's state and repository, `mergeStateStatus` and `closingIssuesReferences`. They come in the existing searches, with no extra request.
- **No ranking.** Labels come as they are named; each team writes its rule where its agent reads it. The order of `next` doesn't change.
- **`blocked` is a fact, not a judgment:** at least one blocking issue is open.
- **Dependency text in issue bodies is not parsed.** The docs show how to turn it into a native relation.
- **The text shows only open blockers and actionable merge states.** `UNKNOWN` (still being computed) and `DRAFT` (already shown) add nothing; unknown values are shown as they come.
- **Limits of 20 labels, 20 blockers and 10 closed issues,** with a warning when GitHub counts more, which also covers blockers the viewer can't see.

## Alternatives considered

- **Parsing "Depends on: #N" from bodies:** works for one team's wording and fails silently for the rest.
- **Ranking by a configured priority label:** a setting per team for something the agent can do from one line in CLAUDE.md.

## Consequences

- On cli/cli, a search page of 25 issues and 25 pull requests went from about 2.8 to 3.9 seconds. `next` only reads the viewer's work, which is usually a few items.
- `mergeStateStatus` can be `UNKNOWN` on the first read after a push.
- Teams that track dependencies only in text see `blocked: false` until they add native relations.
