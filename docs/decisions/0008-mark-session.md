# 0008: Letting an agent move the session

## Context

Decision 0003 lets only the text report move the session, so an agent reading `--json` can't hide news from a person. On first real use, an agent was the only reader: a session-start hook and a skill ran `today --json`, the session never moved, and the "what changed" window kept growing.

## Decision

- **`--mark` moves the session from `--json`,** with the same rules as the text report: the saved time is the start of the run, and nothing is saved after a failure, without the Project, or with `--since`.
- **Opt-in, not the default:** a hook that runs on every session start would otherwise move the session even when nobody reads the report.
- **Mistakes are errors:** `--mark` without `--json` would do nothing new, and with `--since` it would contradict it, so both stop before fetching.

## Alternatives considered

- **Moving the session on every `--json` run:** the problem 0003 avoided.
- **A separate command that marks after the report is shown:** no gap between fetching and showing, but a second call the agent has to remember.

## Consequences

- If an agent marks and then fails before showing the report, that window is skipped next time.
- The JSON output doesn't change, so `schemaVersion` stays `1`.
