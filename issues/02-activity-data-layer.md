### Context

This is the core of the tool: fetching *facts* from GitHub. Every other command builds on it.

### Acceptance criteria

- [ ] Package `internal/activity` exposes `Fetch(ctx, repo, since) (Activity, error)`
- [ ] `Activity` includes: PRs merged since and PRs opened since (number, title, author, URL), reviews submitted since (reviewer, state, URL), comments since (author, excerpt, URL, plus the number and author of the parent issue or PR), and failed check runs on the default branch since
- [ ] Comments cover both conversation comments and inline review comments
- [ ] `Activity` includes the authenticated user's login and the open PRs currently requesting their review, regardless of when those PRs were opened
- [ ] Every returned item includes its URL
- [ ] Results beyond 100 items are paginated correctly
- [ ] Bot accounts (author `__typename` is `Bot`) are excluded by default
- [ ] Uses GraphQL through `go-gh` with the minimum number of queries needed (document the queries in a comment)
- [ ] Unit tests use recorded JSON fixtures — no network calls in tests
- [ ] A short `docs/decisions/0002-activity-queries.md` explains why each query is shaped the way it is

### Notes and hints

- Explore queries manually first: `gh api graphql -f query='...'`
- The GitHub GraphQL Explorer is useful for discovering fields.
- The `search` connection with queries like `repo:OWNER/NAME is:pr merged:>=DATE` can cover several needs at once.
- GraphQL reports bots as `dependabot`, not `dependabot[bot]` (that suffix is REST only), so match on `__typename`.
- `search` returns at most 1,000 results, so decide what happens when a query hits that limit.
- `author` is null for deleted accounts.
- Depends on: Scaffold.

### Out of scope

- Project board data (separate issue).
- Any formatting or output.
