# 0004: Reading GitHub Projects in `today`

## Context

`today` should show what changed on the team's board. The API has limits that shape how: items can only be ordered by position, and there is no status history.

## Decision

- **Report "updated", not "moved":** `ProjectV2Item` exposes `updatedAt` and the current field values, but no history, and `updatedAt` changes with any edit. The report shows the current status only.
- **Filter on the server with `items(query: "updated:>=DATE")`:** on a public board of 585 items it returned the same 191 as counting them by hand. The filter takes dates only, and a timestamp returned nothing, so it starts a day early and the exact comparison happens in code.
- **`repositoryOwner` for both kinds of owner:** one query covers users and organizations. The project has to belong to the repository's owner.
- **`.github/receipts.yml` is read through the API,** from the default branch, so it also works with `--repo` outside a clone. `--project` takes priority.
- **Nothing is requested without a project:** the section and the `read:project` scope are optional.
- **A missing scope doesn't stop the report:** the configuration is shared, so a teammate without the scope still gets everything else, a warning with the exact fix, and an unchanged last session. A wrong project number does stop it, because it needs fixing.
- **Items without their own page link to the board:** draft issues have no URL, and neither do items the token can't read.

## Alternatives considered

- **Reading every item and filtering in code:** works, but costs a page per 100 items on every run.
- **Reading `.github/receipts.yml` from the working tree:** fails outside a clone and depends on the checked-out branch.
- **Stopping on a missing scope:** simpler, but one missing scope would block the whole report for that person.

## Consequences

- `today` makes one extra request to read the configuration when `--project` isn't passed.
- Projects owned by someone other than the repository's owner aren't supported.
