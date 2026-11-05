package project

const (
	// projectQuery reads one page of a board. repositoryOwner covers projects
	// owned by users and by organizations. Items can only be ordered by their
	// position on the board, so the filter is what keeps large boards cheap.
	projectQuery = `query ProjectItems($owner: String!, $number: Int!, $filter: String!, $cursor: String) {
  repositoryOwner(login: $owner) {
    ... on User { projectV2(number: $number) { ...projectItems } }
    ... on Organization { projectV2(number: $number) { ...projectItems } }
  }
}
fragment projectItems on ProjectV2 {
  title
  url
  items(first: 100, after: $cursor, query: $filter) {
    pageInfo { hasNextPage endCursor }
    nodes {
      updatedAt
      fieldValueByName(name: "Status") {
        ... on ProjectV2ItemFieldSingleSelectValue { name }
      }
      content {
        __typename
        ... on Issue { number title url assignees(first: 20) { nodes { login } } }
        ... on PullRequest { number title url assignees(first: 20) { nodes { login } } }
        ... on DraftIssue { title assignees(first: 20) { nodes { login } } }
      }
    }
  }
}
`
)
