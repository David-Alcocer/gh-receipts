### Context

Two documentation gaps found in real use:

1. **Agent context goes stale during a long session.** A team's session-start hook saved `next --json` to a file. After the agent merged pull requests and closed issues in that same session, it kept reading the old file and listing closed issues as pending. `gh receipts next --json` and `context` always read live data; the stale part was the saved file. `docs/agent-hook.md` doesn't say how to keep the agent's view fresh.
2. **The "Merged without review" link in `health` can list more pull requests than the count.** `docs/health-json.md` says the search "can list fewer PRs than `count`", but the same paragraph says it includes PRs authored by bots, which the count excludes, so it can also list more. In real use it listed one more than the count. A second possible cause was reported: GitHub's `review:none` may also match pull requests whose only reviews are comments.

### Acceptance criteria

- [ ] `docs/agent-hook.md` has a section on keeping the context fresh during a session: run `gh receipts next --json` or `context` again after merging or closing, instead of reading a saved file, with an example instruction for a team's agent file and an example Claude Code hook that runs after `gh pr merge` or `gh issue close`
- [ ] The example hook is tested once in Claude Code and the result is noted in the PR
- [ ] `docs/health-json.md` says the search link can list more or fewer pull requests than the count, gives each cause, and points to `pullRequests` as the exact list
- [ ] The comment-only review case is checked against GitHub on a real pull request, and the docs state what was found, not a guess

### Out of scope

- Changing how `health` counts reviews.
- A new command to refresh a saved file.
