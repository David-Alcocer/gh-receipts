# 0005: Parsing acceptance criteria

## Context

`next` and `verify` depend on reading the criteria of an issue. The body is free Markdown written by people, and the template in `.github/ISSUE_TEMPLATE/feature.yml` renders each field as a `###` heading. Issue 14 was created with that form to confirm the format: `\n` line endings, and `_No response_` in empty fields.

## Decision

- **A line-by-line parser without dependencies:** the format only needs headings, checkboxes and code blocks.
- **The first section wins,** and it ends at the next heading of the same or a higher level, so `####` groups stay inside a `###` section.
- **Checkboxes in code blocks don't count,** because they are examples.
- **Empty boxes don't count:** the template starts with `- [ ]`, so an issue left unfilled would read as `0/1`.
- **Missing and empty are different results:** both are answers and not errors, but `found` separates "nobody wrote this section" from "it has nothing yet".
- **Nested items count as criteria** and keep a `depth`, so a total like `2/5` stays simple.
- **Every text line links to the issue:** GitHub has no link for a single checkbox, and every line that reports a fact needs a source.

## Alternatives considered

- **A Markdown library:** more accurate, but a new dependency, and it would still need rules for these same cases.
- **Regular expression over the whole body:** shorter, but it can't track code blocks or nesting.
- **Counting only top-level items:** a nested criterion would disappear from the total.

## Consequences

- Only `#` headings are recognized, not underlined ones.
- Criteria spanning several lines keep their first line only.
- Depth counts nested checkboxes, so a checkbox under a plain list item is top-level.
- Checkboxes inside HTML comments are read as criteria.
- `checked` is a claim someone made, not evidence. Checking it against evidence belongs to `verify`.
