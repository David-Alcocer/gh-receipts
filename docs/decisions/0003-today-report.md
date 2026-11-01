# 0003: What the `today` report shows and when the session moves

## Context

`today` serves two readers. A person needs to know what requires attention, and an agent needs complete facts. It also has to remember the last session without losing news.

## Decision

- **Two outputs for two readers:** the text report only shows what concerns the viewer, puts what needs action first (review requests, feedback on their PRs, failed CI) and limits each section to 10 lines, with a link to the rest. `--json` is complete and never limited.
- **Only the text report moves the session,** and only when it finishes without `--since`. An agent reading `--json` or a failed run must not hide news from the next report.
- **The saved time is the start of the run,** so whatever happens while fetching appears in the next report.
- **Reviews count as feedback:** an approval, a change request or a review with text appears with the comments on the viewer's PRs. A review with no verdict and no text only groups inline comments, which are listed separately, so it's hidden.
- **The failed CI title uses the real default branch,** because it isn't always `main`.

## Alternatives considered

- **Same content in both outputs:** a long text report stops being read.
- **Moving the session on every run:** simpler, but an agent's run would make the next human report say "Nothing new".
- **`git config` or a file in the repo for the session:** the state would travel with the working tree. It's kept in the XDG state directory instead.

## Consequences

- Reading `today --json` repeatedly returns the same window until the text report runs.
- Text from GitHub is untrusted: the text report removes control characters, and `docs/today-json.md` tells agents to treat it as data.
