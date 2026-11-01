package activity

// The package needs three queries. docs/decisions/0002-activity-queries.md
// explains why each one has this shape.
//
//  1. activityQuery: one search for everything updated since a timestamp. Merged
//     and opened PRs are derived from its results, and the reviews and comments
//     are read from each PR or issue.
//  2. reviewRequestsQuery: the viewer and the open PRs requesting their review.
//     These can predate the timestamp, so activityQuery may not return them.
//  3. checksQuery: commits on the default branch since the timestamp, with their
//     checks.
//
// The *PageQuery queries continue a nested list that didn't fit in one of the
// queries above. They only run when that list has more pages.

const (
	// A search page can't nest 100 threads with 100 comments each: GitHub rejects
	// queries that could return more than 500,000 nodes, so threads and their
	// comments use smaller pages. The search page has 25 results because busy
	// repositories answered 100 with HTTP 502 and 504.
	activityQuery = `query Activity($search: String!, $cursor: String) {
  search(query: $search, type: ISSUE, first: 25, after: $cursor) {
    issueCount
    pageInfo { hasNextPage endCursor }
    nodes {
      __typename
      ... on PullRequest {
        id
        number
        title
        url
        createdAt
        mergedAt
        author { __typename login }
        reviews(last: 100) { ...reviewPage }
        comments(last: 100) { ...commentPage }
        reviewThreads(last: 50) { ...threadPage }
      }
      ... on Issue {
        id
        number
        title
        url
        createdAt
        author { __typename login }
        comments(last: 100) { ...commentPage }
      }
    }
  }
}
` + activityFragments

	activityFragments = `fragment reviewPage on PullRequestReviewConnection {
  pageInfo { hasPreviousPage startCursor }
  nodes { state submittedAt url author { __typename login } }
}
fragment commentPage on IssueCommentConnection {
  pageInfo { hasPreviousPage startCursor }
  nodes { url bodyText createdAt author { __typename login } }
}
fragment threadPage on PullRequestReviewThreadConnection {
  pageInfo { hasPreviousPage startCursor }
  nodes {
    id
    comments(last: 20) { ...threadCommentPage }
  }
}
fragment threadCommentPage on PullRequestReviewCommentConnection {
  pageInfo { hasPreviousPage startCursor }
  nodes {
    url
    bodyText
    createdAt
    author { __typename login }
    pullRequestReview { url }
  }
}
`

	reviewsPageQuery = `query ReviewsPage($id: ID!, $cursor: String) {
  node(id: $id) {
    ... on PullRequest { reviews(last: 100, before: $cursor) { ...reviewPage } }
  }
}
fragment reviewPage on PullRequestReviewConnection {
  pageInfo { hasPreviousPage startCursor }
  nodes { state submittedAt url author { __typename login } }
}
`

	commentsPageQuery = `query CommentsPage($id: ID!, $cursor: String) {
  node(id: $id) {
    ... on PullRequest { comments(last: 100, before: $cursor) { ...commentPage } }
    ... on Issue { comments(last: 100, before: $cursor) { ...commentPage } }
  }
}
fragment commentPage on IssueCommentConnection {
  pageInfo { hasPreviousPage startCursor }
  nodes { url bodyText createdAt author { __typename login } }
}
`

	threadsPageQuery = `query ThreadsPage($id: ID!, $cursor: String) {
  node(id: $id) {
    ... on PullRequest { reviewThreads(last: 50, before: $cursor) { ...threadPage } }
  }
}
fragment threadPage on PullRequestReviewThreadConnection {
  pageInfo { hasPreviousPage startCursor }
  nodes {
    id
    comments(last: 20) { ...threadCommentPage }
  }
}
fragment threadCommentPage on PullRequestReviewCommentConnection {
  pageInfo { hasPreviousPage startCursor }
  nodes {
    url
    bodyText
    createdAt
    author { __typename login }
    pullRequestReview { url }
  }
}
`

	threadCommentsPageQuery = `query ThreadCommentsPage($id: ID!, $cursor: String) {
  node(id: $id) {
    ... on PullRequestReviewThread {
      comments(last: 100, before: $cursor) {
        pageInfo { hasPreviousPage startCursor }
        nodes {
          url
          bodyText
          createdAt
          author { __typename login }
          pullRequestReview { url }
        }
      }
    }
  }
}
`

	reviewRequestsQuery = `query ReviewRequests($search: String!, $cursor: String) {
  viewer { login }
  search(query: $search, type: ISSUE, first: 100, after: $cursor) {
    issueCount
    pageInfo { hasNextPage endCursor }
    nodes {
      ... on PullRequest {
        number
        title
        url
        author { __typename login }
      }
    }
  }
}
`

	checksQuery = `query Checks($owner: String!, $name: String!, $since: GitTimestamp!, $cursor: String) {
  repository(owner: $owner, name: $name) {
    defaultBranchRef {
      target {
        ... on Commit {
          history(first: 100, since: $since, after: $cursor) {
            pageInfo { hasNextPage endCursor }
            nodes {
              id
              oid
              url
              statusCheckRollup {
                state
                contexts(first: 100) { ...contextPage }
              }
            }
          }
        }
      }
    }
  }
}
` + contextFragment

	checkContextsPageQuery = `query CheckContextsPage($id: ID!, $cursor: String) {
  node(id: $id) {
    ... on Commit {
      statusCheckRollup {
        contexts(first: 100, after: $cursor) { ...contextPage }
      }
    }
  }
}
` + contextFragment

	contextFragment = `fragment contextPage on StatusCheckRollupContextConnection {
  pageInfo { hasNextPage endCursor }
  nodes {
    __typename
    ... on CheckRun { name conclusion url }
    ... on StatusContext { context state targetUrl }
  }
}
`
)
