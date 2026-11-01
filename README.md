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

| Command | What it does | Status |
|---|---|---|
| `gh receipts today` | What changed since your last session: merged PRs, reviews waiting for you, new comments on your PRs, failed CI, Project board movement | planned (v0.1) |
| `gh receipts criteria <issue>` | Parses an issue's acceptance criteria checklist | planned (v0.2) |
| `gh receipts health` | Team flow signals: PR size, time to first review, time to merge, stale PRs, issues opened vs closed | planned (v0.2) |
| `gh receipts verify <pr>` | Checks each acceptance criterion of the linked issue against evidence: *met / partial / no evidence* | planned (v0.3) |

### Example (target output for `today`)

```
$ gh receipts today
Since Sat 31 Oct, 18:40 (last session)

MERGED
  #42 Add login form validation (ana)            github.com/acme/app/pull/42

WAITING FOR YOUR REVIEW
  #45 Persist sessions in Redis (luis)           github.com/acme/app/pull/45

NEW COMMENTS ON YOUR PRS
  #44 "Can we move this to a hook?" (ana)        github.com/acme/app/pull/44#discussion_r123

FAILED CI ON MAIN
  test / unit  @ 3f2a1c9                          github.com/acme/app/actions/runs/987

PROJECT
  #39 Signup flow  Review                         github.com/orgs/acme/projects/3
```

### Feeding your agent facts instead of memories

```
mkdir -p .receipts
gh receipts today --json > .receipts/today.json
```

Then ask your coding agent to plan the day from that file. The agent works from verifiable data, and you can check every claim.

## Install

```
gh extension install David-Alcocer/gh-receipts
```

Requires the GitHub CLI (`gh`) authenticated. Project board features need the `read:project` scope:

```
gh auth refresh -s read:project
```

## How we work

This repo is built the way it recommends working:

- **Trunk-based development:** short-lived branches, small PRs, `main` always green.
- **Every issue has acceptance criteria** written as a checklist (see `.github/ISSUE_TEMPLATE/feature.yml`).
- **Every PR links its issue** with `Closes #N`.
- **Built with an AI coding agent, owned by humans:** if a PR can't be explained line by line by its author, it doesn't get merged.

## Contributing

Issues labeled `feature` have explicit acceptance criteria — pick one, comment that you're taking it, and open a PR that closes it. Issues labeled `hand-written` are reserved for the maintainer to write without AI generation.

## License

MIT
