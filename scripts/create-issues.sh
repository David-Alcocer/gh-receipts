#!/usr/bin/env bash
# Creates the labels and the initial issues for gh-receipts.
# Run once, on a repo with no issues or PRs: running it again duplicates the
# issues, and ROADMAP.md assumes they get numbers #1 to #6.
# Review and edit issues/*.md first, then run from the repo root:
#
#   ./scripts/create-issues.sh
#   PROJECT="gh-receipts" ./scripts/create-issues.sh   # also add issues to a Project (by title)
#
# Requires: gh authenticated with access to this repo
# (adding to a Project needs the "project" scope: gh auth refresh -s project).
set -euo pipefail

PROJECT="${PROJECT:-}"

gh label create feature      --color 1D76DB --description "Unit of work with acceptance criteria" --force
gh label create mvp          --color 0E8A16 --description "Part of v0.1 or v0.2" --force

create() {
  local file="$1" title="$2" labels="$3"
  local args=(--title "$title" --body-file "$file" --label "$labels")
  if [[ -n "$PROJECT" ]]; then
    args+=(--project "$PROJECT")
  fi
  gh issue create "${args[@]}"
}

create issues/01-scaffold.md                 "Scaffold gh extension with CI and releases"        "feature,mvp"
create issues/02-activity-data-layer.md      "Activity data layer: PRs, reviews, comments, CI"   "feature,mvp"
create issues/03-today-command.md            "today command with session memory and --json"     "feature,mvp"
create issues/04-project-section.md          "Project board section in today"                   "feature,mvp"
create issues/05-acceptance-criteria-parser.md "Acceptance criteria convention and parser"      "feature,mvp"
create issues/06-health-command.md           "health command with team-level flow signals"      "feature,mvp"

echo "Done. Issues created."
