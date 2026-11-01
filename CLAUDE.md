# CLAUDE.md

Instructions for AI coding agents working in this repo.

## What this project is

`gh-receipts` is a GitHub CLI extension (Go) that reports what a team actually shipped, using facts from Git and GitHub. Core principle: **every claim has a source, and "no evidence" is a valid answer.** Apply the same principle to your own work here.

## Language

- Talk to the maintainer in **Spanish**.
- Write code, comments, commits, PRs, issues and docs in **English**.

## Starting a session

Before answering "where did we leave off?", get facts, don't rely on memory:

1. `git status` and `git log --oneline -10`
2. `gh issue list --assignee @me`
3. `gh pr list --author @me`

(Once `gh receipts today` works, use it instead.)

Summarize only what those commands returned, with issue/PR numbers.

## Working on an issue

1. Read it: `gh issue view <N>`. Restate its acceptance criteria as a numbered list.
2. **Propose a plan and stop.** Do not write code until the maintainer approves the plan.
3. Work on a short-lived branch: `feat/<N>-short-name` (or `fix/`, `chore/`). Never commit to `main`.
4. Small commits using Conventional Commits (`feat:`, `fix:`, `test:`, `docs:`, `chore:`).
5. Open the PR with `gh pr create`. The PR description must include:
   - `Closes #<N>`
   - A table mapping **each acceptance criterion** to its evidence (file and line, test name, or CI link), marked `met`, `partial` or `not done`. Never mark something `met` without evidence.
   - "Things to understand before merging": the 3–5 key decisions or tricky parts the maintainer must be able to explain.

## Issues labeled `hand-written`

The maintainer writes the implementation. You may explain concepts, answer questions, suggest test cases, review code the maintainer wrote and point to docs. **Do not write the implementation code**, even if asked casually; remind the maintainer of this rule instead.

## Technical rules

- Go, with `github.com/cli/go-gh/v2` for auth and API clients. No manual token handling.
- Tests never hit the network: use recorded JSON fixtures in `testdata/`.
- Keep data fetching separate from rendering.
- Every user-facing output line that reports a fact must include its URL.

## Decisions

Record significant decisions in `docs/decisions/NNNN-short-title.md` (context, decision, alternatives considered, consequences). Keep each one short.

## Ask before you

- Add a dependency
- Change an issue's acceptance criteria
- Change CI, release or branch protection settings
- Push, merge a PR or close an issue