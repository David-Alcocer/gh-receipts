# 0007: Spanish headings and plain list criteria

## Context

Decision 0005 only reads checkboxes under "Acceptance criteria". On first real use, a team writing issues in Spanish had "Criterios de aceptación" sections with plain `-` items, and `criteria` answered "no criteria found" for every issue. That answer was false.

## Decision

- **"Criterios de aceptación" is a section name too,** with or without the accent, in any case.
- **A section without checkboxes uses its plain list items.** A section with checkboxes keeps only those, so notes under a checklist item don't become criteria.
- **A plain item is not unchecked:** it has `checkbox: false`, and the text report shows it with `-` and says "N criteria, no checkboxes" instead of `0/N`. Nobody claimed it's not done.
- **`checkbox` is a new field, so `schemaVersion` stays `1`:** existing fields keep their meaning for checkbox items.

## Alternatives considered

- **Count plain items as unchecked boxes:** simpler, but `0/3` would claim something nobody wrote.
- **Always count plain items, with or without checkboxes:** explanations nested under a checkbox would be counted as criteria.
- **A configurable list of heading names:** more general, but nobody has asked for a third language yet.

## Consequences

- A section mixing both kinds reports only its checkboxes.
- Malformed boxes like `- [y]` are skipped, not read as plain text.
- A plain item can't say whether it's done. Checking it against evidence belongs to `verify`.
