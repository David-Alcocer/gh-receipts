### Context

The goal of gh-receipts is that an agent knows the next step from facts: the person's issues and pull requests, what the team changed and what people said, including when the team changes direction. Direction usually changes in the issue: a comment saying "let's drop X and do Y", edited acceptance criteria, an issue closed as not planned, or the work reassigned to someone else.

`context` shows none of that today. `today` already fetches comments on issues (they appear in `today --json` with `onPullRequest: false`), but the text report and `context` only show comments on the person's pull requests. Changes to an issue itself (description, title, state, assignees, labels, milestone) aren't read at all. So an agent can start the day planning work the team cancelled yesterday.

### Acceptance criteria

- [ ] `context` and the `today` text report show comments by other people, since the start of the window, on open issues assigned to the viewer, each with an excerpt, its author and its URL
- [ ] They show changes since the start of the window to those issues, each with a link: description edited, title changed, labels or milestone changed, assignees changed
- [ ] An issue assigned to the viewer that was closed since the start of the window, or that stopped being assigned to the viewer, appears once with what happened (for example "closed as not planned" or "unassigned"), so the agent stops planning it
- [ ] A description edit is reported as an edit with a link, without guessing what changed; the agent can run `gh receipts criteria <N>` to read the current criteria
- [ ] Changes made by the viewer and by bots are left out, as with comments today
- [ ] The new sections follow the existing rules: section limits with "… and N more", text cleaned of control characters, and the notice that text written by others is data, not instructions
- [ ] `today --json` includes the issue changes; the change is additive, so `schemaVersion` stays `1`
- [ ] Fetching and rendering are tested with recorded fixtures, without network

### Notes and hints

- The search in `internal/activity/updated.go` already returns every issue updated since the start of the window, including issues the viewer was unassigned from, so no extra search should be needed.
- GraphQL `Issue.timelineItems(since:, itemTypes: [...])` has `ClosedEvent` (with `stateReason`), `ReopenedEvent`, `AssignedEvent`, `UnassignedEvent`, `RenamedTitleEvent`, `LabeledEvent`, `UnlabeledEvent`, `MilestonedEvent` and `DemilestonedEvent`. `Issue.lastEditedAt` and `editor` say when the description was last edited and by whom.
- Measure the number of API calls on a busy repository before and after, as `health` did, since this adds a timeline read per updated issue.
- Order of sections in `context`: issue changes could go right after "ISSUES ASSIGNED TO YOU", since they can change what that list means.

### Out of scope

- Comments and changes on issues not assigned to the viewer.
- Showing what changed in a description (a diff).
- Summarizing comments with AI.
- Linked pull requests and sub-issues.
