### Context

In `criteria --json`, a plain list item (no checkbox) has `checked: false` and `checkbox: false`. Decision 0007 documented that `checkbox: false` means the item can't be checked. In practice, agents in two different teams read `checked: false` as "not done", because they read the data, not the docs. The field misleads the readers it is for.

The same items appear inside `next --json`, under each issue's `criteria`.

### Acceptance criteria

- [ ] For a plain list item, `checked` is `null` in `criteria --json` and in `next --json`; for a checkbox it stays `true` or `false`
- [ ] `schemaVersion` goes from `1` to `2` in `criteria --json` and in `next --json`, because a field changes meaning
- [ ] `docs/criteria-json.md` and `docs/next-json.md` document the new value and what changed from version 1, so existing readers can update
- [ ] The text output doesn't change: plain items still show `-` and "N criteria, no checkboxes"
- [ ] A decision record explains why 0007 is revised
- [ ] Tests cover both kinds of item in both commands, without network

### Notes and hints

- `criteria.Item` is shared by `criteria` and `next`, so one type change reaches both outputs.
- `checkbox` stays as it is, so readers can still tell the two kinds apart without looking at `checked`.

### Out of scope

- Telling whether a criterion is really met (that's `verify`).
- Changes to how criteria are parsed.
