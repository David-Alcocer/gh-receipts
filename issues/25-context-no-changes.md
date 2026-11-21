### Context

`gh receipts context` leaves out empty sections. When there is pending work but nothing changed since the last session, the summary shows only the pending sections, and the reader can't tell "nothing changed" from "changes weren't read". In the tool's own terms, "no evidence" is a valid answer, so it should be said, not implied.

First use in another team's repository also showed two gaps in `docs/agent-hook.md`:

- The rule "It never fails" doesn't say that it assumes the extension is installed. The exception appears later in the page.
- That team's own hook gives the agent only counts (how many reviews, PRs, issues), so untrusted titles and comments never reach the agent's context at startup. The docs don't mention that option.

### Acceptance criteria

- [ ] When the changes were read and none of the "what changed" sections has anything, `context` prints one line saying nothing changed since the start of the window, with a link where that can be checked
- [ ] When the changes couldn't be read, that line doesn't appear; the existing `Note:` line explains it
- [ ] The existing "Nothing new and nothing pending" line still covers the case where both parts are empty
- [ ] `docs/agent-hook.md` says, in the "It never fails" rule itself, that it needs the GitHub CLI and the extension installed
- [ ] `docs/agent-hook.md` describes the counts-only option for repositories where untrusted text in the context is a concern, and what it costs (the agent has to fetch the details itself)
- [ ] Rendering tests cover each case without network

### Notes and hints

- `internal/report/context.go`, `ContextText`: the change sections come from `changeSections`, so emptiness can be checked there.
- The link could be the repository's pull request search for anything updated since the start of the window.
- Counts-only is documentation only in this issue. A `--counts-only` flag waits until a second team asks for it.

### Out of scope

- A `--counts-only` flag.
- Changing `today` or `next` output.
