# 0016: `next` leaves out what the search returns after it was closed

## Context

Issue #40. `next` and `context` find the viewer's open pull requests, review requests and assigned issues with GitHub searches (`is:open`). Testing the hook from #35, an issue closed by the agent stayed in that search for about 5 seconds, so a `next` run right after `gh issue close` still listed it as pending. During those seconds each result already had its real state: the closed issue came back with `state: CLOSED`.

## Decision

- **Ask for `state` in the three searches and keep only `OPEN` results.** The search decides which items to look at; the item's own state decides whether it's still pending.
- **Review requests too, although that search is shared with `today`,** which #40 left out of scope. The maintainer chose to include it: it's the same fix, and `today` stops listing reviews on pull requests that were just merged.
- **No new warning.** An item left out was closed or merged, which is a fact and not a gap in the data.
- **The recorded fixtures gain `"state": "OPEN"`.** They come from `is:open` searches, so that's what GitHub returned; the field didn't exist in the query when they were recorded.

## Alternatives considered

- **Wait or retry before searching:** slower every time, and 5 seconds was one measurement, not a guarantee.
- **Read each item again outside the search:** one more request per item for the same answer the search already gives.

## Consequences

- An item reopened a moment ago can be missing for the same few seconds, because the search hasn't indexed it as open yet. Nothing here can see it.
