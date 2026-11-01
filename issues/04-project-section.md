### Context

The team plans in a GitHub Project, so `today` should show what moved on the board without opening the browser.

### Acceptance criteria

- [ ] `today` includes a "Project" section with items updated since the last session, showing the current Status field value and a link
- [ ] The project is selected with `--project <number>` or a `project` key in `.github/receipts.yml`
- [ ] Items assigned to the current user are listed first
- [ ] If the token lacks the `read:project` scope, the output says exactly how to fix it: `gh auth refresh -s read:project`
- [ ] Works for both user-owned and organization-owned projects
- [ ] Included in `--json` output under a `project` key
- [ ] Tested with recorded fixtures

### Notes and hints

- Projects (v2) are only available through the GraphQL API.
- `ProjectV2Item` exposes `updatedAt` and current `fieldValues`, but no status change history, and `updatedAt` isn't specific to Status changes. Report "updated", not "moved"; record the decision in `docs/decisions/`.
- Depends on: `today` command.

### Out of scope

- Moving items or editing fields (read-only for now).
- A visual kanban board (planned for the TUI).
