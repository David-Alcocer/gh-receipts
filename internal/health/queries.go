package health

// Search pages have 25 results because busy repositories answered 100 with HTTP
// 502 and 504.
const (
	// repositoryExistsQuery exists because searching a repository that doesn't
	// exist returns no results instead of an error, which would read as "no
	// data".
	repositoryExistsQuery = `query RepositoryExists($owner: String!, $name: String!) {
  repository(owner: $owner, name: $name) { id }
}
`

	// mergedPullRequestsQuery reads reviews oldest first, so the first one by
	// someone else is usually on the first page.
	mergedPullRequestsQuery = `query MergedPullRequests($search: String!, $cursor: String) {
  search(query: $search, type: ISSUE, first: 25, after: $cursor) {
    issueCount
    pageInfo { hasNextPage endCursor }
    nodes {
      ... on PullRequest {
        number
        url
        additions
        deletions
        createdAt
        mergedAt
        author { __typename login }
        reviews(first: 50) {
          pageInfo { hasNextPage }
          nodes {
            submittedAt
            author { __typename login }
          }
        }
      }
    }
  }
}
`

	stalePullRequestsQuery = `query StalePullRequests($search: String!, $cursor: String) {
  search(query: $search, type: ISSUE, first: 25, after: $cursor) {
    issueCount
    pageInfo { hasNextPage endCursor }
    nodes {
      ... on PullRequest {
        number
        title
        url
        isDraft
        updatedAt
        author { __typename }
      }
    }
  }
}
`

	issuesQuery = `query Issues($search: String!, $cursor: String) {
  search(query: $search, type: ISSUE, first: 25, after: $cursor) {
    issueCount
    pageInfo { hasNextPage endCursor }
    nodes {
      ... on Issue {
        number
        createdAt
        closedAt
        author { __typename }
      }
    }
  }
}
`
)
