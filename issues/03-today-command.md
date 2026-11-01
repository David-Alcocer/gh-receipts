### Context

`today` replaces "hey agent, where did we leave off?" with a report where every line links to its source. Its `--json` output is how you feed your coding agent facts instead of relying on its memory.

### Acceptance criteria

- [ ] `gh receipts today` prints these sections: Merged, Waiting for your review, New comments on your PRs, Failed CI on main
- [ ] Empty sections are hidden; if everything is empty, it prints a single "Nothing new since <time>" line
- [ ] Every line ends with a URL — no line is printed without a source
- [ ] The time of the last run is remembered per repo, stored outside the working tree (e.g. under the XDG state dir); default "since" = last run, falling back to 24h on first run
- [ ] `--since` accepts a duration (`48h`) or a date (`2026-11-01`) and overrides the stored value without updating it
- [ ] `--json` prints the same data as JSON, with a stable documented schema
- [ ] `--repo owner/name` works outside a git directory; without it and outside a repo, the error message explains how to fix it
- [ ] Formatting is tested with fixtures from the activity data layer

### Notes and hints

- Depends on: Activity data layer.
- Keep rendering separate from data so the TUI later can reuse the same data.

### Out of scope

- Project board section (separate issue).
- Colors and TUI.
