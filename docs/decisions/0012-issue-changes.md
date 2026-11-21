# 0012: Comments and changes on your issues

## Context

Issue #29. A team usually changes direction in the issue: a comment, edited acceptance criteria, an issue closed as not planned, or the work given to someone else. `today` already read comments on issues, but its text and `context` only showed comments on the viewer's pull requests, and changes to issues weren't read at all. An agent could plan work the team had cancelled.

## Decision

- **Read changes in the same search.** Issue `timelineItems` takes `since`, so the activity query asks each issue for its closes, reopens, assignments, title, label and milestone changes in the window. No request is added per issue.
- **Description edits come from `lastEditedAt` and `editor`.** It's one field and no extra request, but it only knows the last edit and doesn't say what changed. The report says "description edited" with the issue link and leaves reading it to the agent.
- **"Your issues" are assigned to you now, or unassigned from you in the window,** so an issue taken from you is reported once instead of silently leaving your list.
- **One section, `CHANGES ON YOUR ISSUES`, newest first,** mixing comments and changes, because a change of direction is often both. In `context` it follows "ISSUES ASSIGNED TO YOU"; in `today`, the reviews waiting for you.
- **The JSON keeps every issue and change, and the report filters,** as with comments on pull requests. Bots and the viewer's own changes are left out of the text.
- **More than 50 changes on one issue in the window is a warning,** not another page, as with reviews.

## Alternatives considered

- **A timeline request per updated issue:** complete, but one request per issue on busy repositories.
- **Edit history (`userContentEdits`) to show what changed in a description:** exact, but larger and diff-shaped; `gh receipts criteria` already shows the current criteria.
- **Two sections, comments and changes:** easier to scan by type, harder to read as a story.

## Consequences

- On cli/cli with a 7-day window, `today --json` went from about 12 to about 15.5 seconds. With the default window of one session, it is much smaller, but `context` stops at 20 seconds, so very long windows on busy repositories are closer to that limit.
- Events other than closing have no link of their own, so they point to the issue.
- An edit by someone else followed by your own edit is not reported.
