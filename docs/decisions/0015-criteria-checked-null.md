# 0015: `checked` is `null` for plain list items

## Context

Issue #34. Decision 0007 gave plain list items `checked: false` and a new `checkbox: false` field that says they can't be checked. Agents in two different teams read `checked: false` as "not done": they read the data, not the docs. The field misled the readers it was written for, and `criteria` and `next` are meant to be read by agents.

## Decision

- **A plain item has `checked: null`.** Checkboxes keep `true` or `false`. `checkbox` stays, so the two kinds can still be told apart without `checked`.
- **`schemaVersion` goes to `2` in `criteria --json` and `next --json`,** because a field changes meaning. `today` and `health` don't include criteria, so they stay at `1`.
- **Only the JSON changes.** Inside the code `Checked` is still a boolean and the text report is the same, so counting and the `-` marker don't change.

This revises the "a plain item is not unchecked" point of 0007: the idea stays, and now the data says it too.

## Alternatives considered

- **Keep `false` and rely on the docs:** already failed with two teams.
- **Drop `checked` for plain items:** an absent field is easier to miss than `null`, and some readers fail on it.
- **Make `Checked` a pointer in the code:** every reader in the code would have to handle "no value", for the same JSON.

## Consequences

- A reader of version 1 that reads `checked` as a boolean must accept `null`. Both JSON docs say what changed.
- Today the only known readers are the maintainer's agents; the `context` hook uses text and isn't affected.
