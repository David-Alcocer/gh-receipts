### Context

`verify` (v0.3) can only check criteria that are written down in a predictable format. This issue sets the convention and builds the parser that everything else will rely on.

### Acceptance criteria

- [ ] `.github/ISSUE_TEMPLATE/feature.yml` is in the repo and used for new issues
- [ ] Package `internal/criteria` parses an issue body: finds a heading named "Acceptance criteria" (`##` or `###`, case-insensitive) and returns its checklist items with their checked state
- [ ] A missing section returns a "no criteria found" result, not an error
- [ ] Table-driven tests cover: issue-form output, hand-written markdown, nested lists, missing section, empty section, and a section followed by another heading
- [ ] `gh receipts criteria <issue>` prints the parsed criteria (debug command), with `--json` support

### Notes and hints

- Issue forms render each field label as a `###` heading in the issue body — check a real issue created from the template to confirm the format.

### Out of scope

- Matching criteria to evidence (that's `verify`, v0.3).
