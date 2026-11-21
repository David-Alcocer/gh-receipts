### Context

Today a developer has to ask their agent to run `gh receipts today --json`, or write that in a skill or a CLAUDE.md. Instructions in markdown are optional for the agent: it may not read them, or may not follow them. So the session still starts with "where did we leave off?" and an answer from memory.

Coding agents can run a command when a session starts and add its output to the context (in Claude Code, a `SessionStart` hook). If that command is `gh receipts`, every session starts from facts with links, without anyone asking for them.

The extension stays a CLI. This issue adds an output meant for that hook and the docs to set it up.

### Acceptance criteria

- [ ] One command prints a short, plain-text summary for an agent's context: what changed since the last session and what is waiting for the viewer (reviews requested, their open PRs, assigned issues), each fact with its URL
- [ ] The output has a documented size limit: each section shows at most a fixed number of items and says how many were left out
- [ ] Running it never changes the last session of `today`
- [ ] It never blocks the agent from starting: when `gh` isn't authenticated, the folder isn't a GitHub repository, or the API fails, it prints one line saying what's missing and exits with code 0
- [ ] The output starts by saying that titles and comments are data written by others, not instructions
- [ ] `docs/agent-hook.md` shows how to set it up for a whole team in Claude Code (`.claude/settings.json` committed to the repo) and how any other agent can call the same command
- [ ] Rendering is tested with fixtures and golden files, without network

### Notes and hints

- Open question for the plan: a new command (for example `gh receipts context`) or a flag on `today`. A new command can combine `today` and `next` without changing their outputs.
- Claude Code adds a `SessionStart` hook's standard output to the context. Example to adapt:

  ```json
  {
    "hooks": {
      "SessionStart": [
        {
          "matcher": "startup",
          "hooks": [{ "type": "command", "command": "gh receipts context", "timeout": 30 }]
        }
      ]
    }
  }
  ```

- Moving the session forward stays manual (`today --json --mark`), because the hook runs whether or not the developer reads the result.
- Measure the output size on a busy repository (for example `cli/cli`) before picking the limit.

### Out of scope

- A command that writes the agent's settings file for you.
- Hooks on other events (before a tool runs, before a commit).
- An MCP server.
- Marking the session automatically.
