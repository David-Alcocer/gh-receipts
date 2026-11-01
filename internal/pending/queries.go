package pending

// Search pages have 25 results because busy repositories answered 100 with HTTP
// 502 and 504. Bodies come with the search, so reading criteria takes no extra
// request per issue.
const (
	// repositoryExistsQuery exists because searching a repository that doesn't
	// exist returns no results instead of an error, which would read as "nothing
	// pending".
	repositoryExistsQuery = `query RepositoryExists($owner: String!, $name: String!) {
  repository(owner: $owner, name: $name) { id }
}
`

	openPullRequestsQuery = `query OpenPullRequests($search: String!, $cursor: String) {
  search(query: $search, type: ISSUE, first: 25, after: $cursor) {
    issueCount
    pageInfo { hasNextPage endCursor }
    nodes {
      ... on PullRequest {
        number
        title
        url
        isDraft
        reviewDecision
        commits(last: 1) {
          nodes {
            commit {
              statusCheckRollup { state }
            }
          }
        }
      }
    }
  }
}
`

	assignedIssuesQuery = `query AssignedIssues($search: String!, $cursor: String) {
  search(query: $search, type: ISSUE, first: 25, after: $cursor) {
    issueCount
    pageInfo { hasNextPage endCursor }
    nodes {
      ... on Issue {
        number
        title
        url
        body
      }
    }
  }
}
`
)
