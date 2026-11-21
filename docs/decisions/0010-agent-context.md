# 0010: Context for agents at session start

## Context

Issue #22. Asking the agent to run `today --json`, or writing that in a skill or a CLAUDE.md, only works when the agent follows the instruction. Agents such as Claude Code can run a command when a session starts and add its output to the context. The hook runs on every session, whether or not anyone reads the result, and its output goes into the context without review.

## Decision

- **A new command, `context`,** instead of a flag on `today`. It combines `today` and `next`, never moves the session and never fails. As a flag it would need errors for `--json`, `--mark` and `--since`, as `--mark` did in 0008.
- **Plain text with the existing sections,** limited to 5 items each instead of 10. Every line keeps its URL and the cleaning of untrusted text.
- **Review requests come from `next`:** all open requests, shown once, instead of only the ones made since the last session.
- **It never moves the session.** 0008 made moving it opt-in for the same reason: a hook doesn't mean someone read the report.
- **Exit code 0 and one line on any failure,** everything on standard output, and partial output when only one of the two fetches fails. A failure at session start shouldn't cost the session.
- **Both fetches run at the same time with a 20-second limit,** under the 30 seconds the docs give the hook.

## Alternatives considered

- **`today --brief`:** one command less, but different rules under the same name.
- **JSON for the agent:** complete, but long, and `today --json` and `next --json` already exist.
- **Moving the session from the hook:** the "what changed" window would stop growing, but changes nobody saw would disappear from the next report.

## Consequences

- Until the developer runs `today` or the agent runs `today --json --mark`, the "what changed" window keeps growing. The limit per section keeps the output short anyway.
- Titles and comments from anyone who can comment reach the agent without review. The output says to treat them as data, which reduces the risk but doesn't remove it.
- A teammate without the extension gets a failed hook and a session without the summary.
