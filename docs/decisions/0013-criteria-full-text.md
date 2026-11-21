# 0013: Criteria in full in the text output

## Context

Issue #27. The text output of `criteria` cut each item to 60 characters, as `today` does with comment excerpts. Reading the criteria is the point of the command, and real criteria are longer: on #29 they reach 248 characters, so more than half of each requirement was hidden.

## Decision

- **Items are shown in full,** wrapped between words at 64 characters, with continuation lines indented under the item's text. A longer word, such as a link, gets its own line instead of being cut.
- **The first line of each item ends with the issue URL; continuation lines don't.** This refines 0005's "every text line links to the issue" to "every item links to the issue": a continuation line is the same fact, not a new one.
- **Columns are padded by hand,** without `text/tabwriter`, because continuation lines have no URL cell and would either break the alignment or carry trailing spaces.
- **`today` keeps cutting comment excerpts,** because there the excerpt is a pointer to the comment, not the content itself.

## Alternatives considered

- **Full text on one line:** the URL column aligns to the longest item, so every line gets as wide as it.
- **The URL only on the summary line:** shorter lines, but items without a source.

## Consequences

- A long item takes several lines, so `criteria` output can be longer than before.
- Width is counted in characters, so wide characters such as emoji can make a line look slightly longer than 64.
