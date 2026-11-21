### Context

`gh receipts criteria <issue>` shortens each item to 60 characters with "…" in the text output, so it fits on one line. Reading the acceptance criteria is the purpose of the command, and acceptance criteria are often longer than 60 characters, so the text output hides part of the requirement. `--json` already has the full text.

### Acceptance criteria

- [ ] The text output of `criteria` shows every item in full
- [ ] Every line that reports an item still ends with the issue URL, or the docs state another way to keep a source on each item
- [ ] Control characters are still removed from item text
- [ ] Other commands keep their current limits: `today` comment excerpts and `next` stay as they are
- [ ] Tests cover a long item, and the existing test that expects shortening is updated

### Notes and hints

- `internal/report/criteria.go`, `CriteriaText` calls `shorten`, which uses `lineExcerptLength` (60) from `internal/report/text.go`. `today` also uses it for comment excerpts, so the change should stay in `criteria`.
- Options for the plan: write the full text and put the URL at the end of the line; wrap long items and put the URL on the first line; or print the URL once in the header line and leave items without it. The last one changes the rule from decision 0005 ("every text line links to the issue") and needs a note in a decision record.
- `--json` doesn't change.

### Out of scope

- The `checked` field for plain list items. Decision 0007 already covers it with `checkbox: false`.
- Changes to the JSON schema.
