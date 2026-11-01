# 0006: The `next` command

## Context

`today` answers "what changed?", so an assigned issue with no recent movement never appears. `next` answers "what's left to do?" from the current state of the repository.

## Decision

- **A separate `internal/pending` package, with no session:** pending work doesn't depend on the last time someone looked, and without a store the command can't change it.
- **`activity` exports its search helpers and review requests:** both commands read review requests the same way, including the filter for bots.
- **Bodies come with the search:** criteria are parsed from each result, with no extra request per issue.
- **A fixed order computed from facts:** failed checks and requested changes go first, and numbers order the rest.
- **Missing data stays missing:** "no CI" and "no review decision" are shown as such, and values the tool doesn't know are printed as they came.
- **"Nothing pending" links the repository,** so that line has a source too.
- **The repository is checked first:** a search on a repository that doesn't exist returns no results and no error, which would read as "Nothing pending".

## Alternatives considered

- **Extending `today`:** it would mix "what changed" with "what's left", and a report tied to the session can't show old work.
- **Ordering by last update:** a bot or a small edit would reorder the list.
- **A request per issue for its criteria:** simpler code, but one more request for each issue.

## Consequences

- `next` makes at least four requests.
- It only covers one repository and issues assigned to the viewer.
- `ciState` describes the latest commit only.
