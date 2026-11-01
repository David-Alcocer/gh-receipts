# 0002: Shape of the activity queries

## Context

`internal/activity` has to report what happened in a repository since a timestamp, with as few GraphQL queries as possible.

## Decision

Use three queries. The rest only run when a nested list has more pages.

### 1. Everything updated since the timestamp

`repo:OWNER/NAME updated:>=SINCE sort:updated-desc` returns issues and PRs together. Anything merged, opened, reviewed or commented on since the timestamp has also been updated, so merged PRs, opened PRs, reviews and comments all come from this one search.

- **Filtered in code:** an update says nothing about a merge. An old PR that was touched recently also matches, and nested lists have no `since` argument, so every date is compared to the timestamp.
- **25 results per page:** pages of 100 got HTTP 502 and 504 from busy repositories. Pages of 25 and 50 worked on `nodejs/node`.
- **Nested list sizes:** 100 threads of 100 comments inside a search page is rejected for exceeding 500,000 possible nodes, so threads use 50 and their comments 20.
- **Read from the newest item (`last`):** older items only matter until the timestamp is passed. Comment lists stop there. Reviews and threads are read completely, because a review can be submitted long after it was started, and an old thread can receive a new comment.
- **Reviews and inline comments are both kept:** GitHub reports a review and the inline comments inside it separately. Inline comments carry `ReviewURL`, so `today` can decide what to show. Timestamps can't relate them: they differed by one second in a recorded example.

### 2. Review requests

`repo:OWNER/NAME is:pr is:open review-requested:@me`, with `viewer`. These PRs may not have been updated since the timestamp, so the first query can't be relied on to return them.

### 3. Checks on the default branch

`defaultBranchRef` avoids assuming the branch is called `main`. `statusCheckRollup.state` is a summary: commits with `SUCCESS` are skipped, and the other commits' contexts are read completely, because a failure can be on any page.

A failed check means:

- `CheckRun`: `FAILURE`, `TIMED_OUT` or `STARTUP_FAILURE`.
- `StatusContext`: `FAILURE` or `ERROR`.

`CANCELLED` isn't a failure, because a run usually gets cancelled when someone stops it or a newer commit replaces it. A commit whose rollup is `FAILURE` only because of a cancelled run reports nothing.

## Alternatives considered

- **One search per kind of activity:** easier to read, but it needs more queries and still has to read nested reviews and comments.
- **An error when a search matches more than 1,000 results:** GitHub returns at most 1,000, but partial data with a warning in `Activity.Warnings` is more useful, as long as the truncation is never silent.
- **`first` for nested lists:** new items would be on the last page.

## Consequences

- Bots are identified by `__typename`, because GraphQL reports `dependabot`, not `dependabot[bot]`. PRs from bots are also left out of review requests.
- `history(since:)` filters by commit date, so a check that fails later on an older commit isn't reported.
- Very busy repositories can still fail: `kubernetes/kubernetes` got an HTTP 502 on the fifth page of a 374-result search. `Fetch` returns the error instead of partial data.
