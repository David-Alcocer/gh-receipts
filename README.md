# gh-receipts

> Know what your team actually shipped. Every claim comes with a receipt.

`gh-receipts` is a [GitHub CLI](https://cli.github.com/) extension for small teams that build with AI coding agents. It answers the questions you ask every morning — *where did we leave off? did my teammate finish their issue? is our flow getting stuck?* — using facts from Git and GitHub, not from a model's memory.

> **Status:** early development. Nothing below is stable yet. See the [roadmap](ROADMAP.md).

## The problem

With agents writing more code than ever, small teams lose track of what actually happened:

- You ask your agent "what did my teammate do?" and get a confident summary you can't verify.
- Every new session starts with "where did we leave off?" and the answer depends on what the agent remembers.
- Review comments, CI failures and board changes are scattered across GitHub tabs.
- There's no simple signal telling you whether your workflow is healthy or getting stuck.

## The principle

1. **Facts come from Git and GitHub.** Commits, PRs, reviews, comments, CI runs and Project items are fetched directly from the API.
2. **Every line has a source.** If something is reported, it links to the commit, comment, review or run that proves it.
3. **"No evidence" is a valid answer.** The tool never fills gaps with guesses.
4. **AI summarizes, it doesn't testify.** Optional AI features may only summarize facts the tool already fetched, and must cite them.

## Commands

| Command | Answers | What it shows | Since |
|---|---|---|---|
| `gh receipts today` | *Where did we leave off?* | What changed since your last session: reviews waiting for you, new comments and reviews on your PRs, failed CI, merged PRs, Project board updates, with `--json` ([schema](docs/today-json.md)) | v0.1.0 |
| `gh receipts next` | *What's next?* | Your open PRs, reviews waiting for you and assigned issues with their criteria, with `--json` ([schema](docs/next-json.md)) | v0.2.0 |
| `gh receipts criteria <issue>` | *What does this issue need?* | An issue's acceptance criteria checklist, with `--json` ([schema](docs/criteria-json.md)) | v0.2.0 |
| `gh receipts health` | *Is our flow getting stuck?* | Team flow signals: PR size, time to first review, PRs merged without review, time to merge, stale PRs, issues opened vs closed, with `--json` ([docs](docs/health-json.md)) | v0.3.0 |
| `gh receipts context` | All of the above, for your agent | A short summary for a coding agent's session-start hook: what is waiting for you and what changed ([docs](docs/agent-hook.md)) | v0.4.0 |
| `gh receipts verify <pr>` | *Did this PR really close its issue?* | Each acceptance criterion of the linked issue against evidence: *met / partial / no evidence* | planned |

### Example: `today`

```
$ gh receipts today
Since Sat 31 Oct, 18:40 (last session)

WAITING FOR YOUR REVIEW
  #45 Persist sessions in Redis (luis)                  https://github.com/acme/app/pull/45

NEW COMMENTS ON YOUR PRS
  #44 "Can we move this to a hook?" (ana)               https://github.com/acme/app/pull/44#discussion_r123
  #43 Changes requested: "Please add a test" (luis)     https://github.com/acme/app/pull/43#pullrequestreview-456

FAILED CI ON MAIN
  test / unit @ 3f2a1c9                                 https://github.com/acme/app/runs/987

MERGED
  #42 Add login form validation (ana)                   https://github.com/acme/app/pull/42

PROJECT
  #39 Signup flow  Review                               https://github.com/acme/app/issues/39
```

## Give your agent facts instead of memories

Run `gh receipts context` from your agent's session-start hook. Every session then starts with your open PRs, reviews waiting for you, assigned issues and what changed, each with its link, and nobody has to ask for them. Developers keep asking in their own words, like "where did we leave off?" or "what's next?", and the agent answers from those facts.

For Claude Code, commit this to `.claude/settings.json` so the whole team gets it:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup",
        "hooks": [
          { "type": "command", "command": "gh receipts context", "timeout": 30 }
        ]
      }
    ]
  }
}
```

`context` never fails a session and never moves your last session. Titles and comments in it are written by other people, so the agent is told to treat them as data. See [Context for coding agents](docs/agent-hook.md) for the rules and other agents.

For the complete data, agents can read JSON instead:

```
gh receipts today --json
gh receipts next --json
```

`today --json` doesn't change your last session; when the agent shows you the report, `today --json --mark` moves it forward.

## Install

```
gh extension install David-Alcocer/gh-receipts
```

Requires the GitHub CLI (`gh`) authenticated. To get a new release:

```
gh extension upgrade receipts
```

### Show a GitHub Project in `today`

Pass `--project <number>`, or set it for everyone in `.github/receipts.yml` on the default branch:

```yaml
project: 3
```

The project has to belong to the repository's owner. Project board features need the `read:project` scope:

```
gh auth refresh -s read:project
```

### Health thresholds

`health` marks a median as over the limit with these defaults, which `.github/receipts.yml` can change ([why these values](docs/health-json.md#thresholds)):

```yaml
health:
  maxPRSize: 400
  maxFirstReviewHours: 24
  maxMergeHours: 48
  staleDays: 2
```

### Install from a local clone

From the repo root (requires Go):

```
go build
gh extension install .
gh receipts --version
```

The installed extension points at this directory, so run `go build` again after each change.

## How we work

This repo is built the way it recommends working:

- **Trunk-based development:** short-lived branches, small PRs, `main` always green.
- **Every issue has acceptance criteria** written as a checklist (see `.github/ISSUE_TEMPLATE/feature.yml`).
- **Every PR links its issue** with `Closes #N`.
- **Built with an AI coding agent, owned by humans:** if a PR can't be explained line by line by its author, it doesn't get merged.

## Contributing

Issues labeled `feature` have explicit acceptance criteria — pick one, comment that you're taking it, and open a PR that closes it.

## License

MIT
