package activity

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"
)

// excerptLength is the most characters a comment excerpt keeps.
const excerptLength = 200

type reviewNode struct {
	State       string
	BodyText    string
	SubmittedAt *time.Time
	URL         string
	Author      *actor
}

type commentNode struct {
	URL       string
	BodyText  string
	CreatedAt time.Time
	Author    *actor
}

type threadCommentNode struct {
	commentNode
	PullRequestReview *struct{ URL string }
}

type threadNode struct {
	ID       string
	Comments backwardPage[threadCommentNode]
}

// updatedNode is a search result: an issue, or a pull request with the extra
// fields only pull requests have.
type updatedNode struct {
	Typename      string `json:"__typename"`
	ID            string
	Number        int
	Title         string
	URL           string
	CreatedAt     time.Time
	MergedAt      *time.Time
	Author        *actor
	Reviews       backwardPage[reviewNode]
	Comments      backwardPage[commentNode]
	ReviewThreads backwardPage[threadNode]
	// Only issues have these.
	State         string
	LastEditedAt  *time.Time
	Editor        *actor
	Assignees     struct{ Nodes []struct{ Login string } }
	TimelineItems backwardPage[timelineNode]
}

// timelineNode is a change to an issue. Each type fills only its own fields.
type timelineNode struct {
	Typename     string `json:"__typename"`
	CreatedAt    time.Time
	Actor        *actor
	URL          string
	StateReason  string
	Assignee     *struct{ Login string }
	CurrentTitle string
	Label        *struct{ Name string }
	// MilestoneTitle is on both MilestonedEvent and DemilestonedEvent.
	MilestoneTitle string
}

// timelineKinds maps GraphQL event types to IssueEvent kinds.
var timelineKinds = map[string]string{
	"ClosedEvent":       EventClosed,
	"ReopenedEvent":     EventReopened,
	"AssignedEvent":     EventAssigned,
	"UnassignedEvent":   EventUnassigned,
	"RenamedTitleEvent": EventRenamed,
	"LabeledEvent":      EventLabeled,
	"UnlabeledEvent":    EventUnlabeled,
	"MilestonedEvent":   EventMilestoned,
	"DemilestonedEvent": EventDemilestoned,
}

// fetchUpdated derives merged PRs, opened PRs, reviews and comments from one
// search for everything updated since the given time.
func fetchUpdated(ctx context.Context, client GraphQLClient, repo repository.Repository, since time.Time) (Activity, error) {
	search := fmt.Sprintf("repo:%s/%s updated:>=%s sort:updated-desc", repo.Owner, repo.Name, since.UTC().Format(time.RFC3339))

	nodes, warnings, err := SearchAll(search, func(cursor interface{}) (SearchConnection[updatedNode], error) {
		var response struct{ Search SearchConnection[updatedNode] }
		variables := map[string]interface{}{"search": search, "cursor": cursor, "since": since.UTC().Format(time.RFC3339)}
		err := client.DoWithContext(ctx, activityQuery, variables, &response)
		return response.Search, err
	})
	if err != nil {
		return Activity{}, err
	}

	result := Activity{Warnings: warnings}
	for _, node := range nodes {
		if err := addNode(ctx, client, &result, node, since); err != nil {
			return Activity{}, fmt.Errorf("read #%d: %w", node.Number, err)
		}
	}
	return result, nil
}

func addNode(ctx context.Context, client GraphQLClient, result *Activity, node updatedNode, since time.Time) error {
	comments, err := fetchComments(ctx, client, node, since)
	if err != nil {
		return err
	}
	result.Comments = append(result.Comments, comments...)

	if node.Typename != "PullRequest" {
		addIssue(result, node, since)
		return nil
	}

	reviews, err := fetchReviews(ctx, client, node, since)
	if err != nil {
		return err
	}
	result.Reviews = append(result.Reviews, reviews...)

	inline, err := fetchInlineComments(ctx, client, node, since)
	if err != nil {
		return err
	}
	result.Comments = append(result.Comments, inline...)

	if node.Author.isBot() {
		return nil
	}
	pr := PullRequest{Number: node.Number, Title: node.Title, Author: node.Author.login(), URL: node.URL}
	if node.MergedAt != nil && !node.MergedAt.Before(since) {
		result.MergedPRs = append(result.MergedPRs, pr)
	}
	if !node.CreatedAt.Before(since) {
		result.OpenedPRs = append(result.OpenedPRs, pr)
	}
	return nil
}

// addIssue stores the issue and its changes by people since the given time.
// The timeline is already limited to the window; the description edit is not,
// and GitHub only says who edited it last.
func addIssue(result *Activity, node updatedNode, since time.Time) {
	assignees := make([]string, 0, len(node.Assignees.Nodes))
	for _, assignee := range node.Assignees.Nodes {
		assignees = append(assignees, assignee.Login)
	}
	result.Issues = append(result.Issues, Issue{Number: node.Number, Title: node.Title, URL: node.URL, State: node.State, Assignees: assignees})

	if node.TimelineItems.PageInfo.HasPreviousPage {
		result.Warnings = append(result.Warnings, fmt.Sprintf("only the last 50 changes of %s since the start of the window were read", node.URL))
	}
	for _, item := range node.TimelineItems.Nodes {
		kind, known := timelineKinds[item.Typename]
		if !known || item.Actor.isBot() {
			continue
		}
		event := IssueEvent{IssueNumber: node.Number, Kind: kind, Actor: item.Actor.login(), Detail: timelineDetail(item), CreatedAt: item.CreatedAt, URL: node.URL}
		if item.URL != "" {
			event.URL = item.URL
		}
		result.IssueEvents = append(result.IssueEvents, event)
	}

	if node.LastEditedAt != nil && !node.LastEditedAt.Before(since) && !node.Editor.isBot() {
		result.IssueEvents = append(result.IssueEvents, IssueEvent{IssueNumber: node.Number, Kind: EventEdited, Actor: node.Editor.login(), CreatedAt: *node.LastEditedAt, URL: node.URL})
	}
}

func timelineDetail(item timelineNode) string {
	switch {
	case item.StateReason != "":
		return item.StateReason
	case item.Assignee != nil:
		return item.Assignee.Login
	case item.CurrentTitle != "":
		return item.CurrentTitle
	case item.Label != nil:
		return item.Label.Name
	}
	return item.MilestoneTitle
}

func fetchReviews(ctx context.Context, client GraphQLClient, node updatedNode, since time.Time) ([]Review, error) {
	// A review can sit pending for days before it's submitted, so its position
	// in the list says nothing about submittedAt and every page has to be read.
	nodes, err := walkBack(node.Reviews, func(reviewNode) bool { return true }, func(cursor string) (backwardPage[reviewNode], error) {
		var response struct {
			Node struct{ Reviews backwardPage[reviewNode] }
		}
		err := client.DoWithContext(ctx, reviewsPageQuery, pageVariables(node.ID, cursor), &response)
		return response.Node.Reviews, err
	})
	if err != nil {
		return nil, err
	}

	var reviews []Review
	for _, review := range nodes {
		if review.SubmittedAt == nil || review.SubmittedAt.Before(since) || review.Author.isBot() {
			continue
		}
		reviews = append(reviews, Review{
			PRNumber:    node.Number,
			PRAuthor:    node.Author.login(),
			Reviewer:    review.Author.login(),
			State:       review.State,
			Excerpt:     excerpt(review.BodyText),
			SubmittedAt: *review.SubmittedAt,
			URL:         review.URL,
		})
	}
	return reviews, nil
}

func fetchComments(ctx context.Context, client GraphQLClient, node updatedNode, since time.Time) ([]Comment, error) {
	nodes, err := walkBack(node.Comments, func(oldest commentNode) bool { return !oldest.CreatedAt.Before(since) }, func(cursor string) (backwardPage[commentNode], error) {
		var response struct {
			Node struct{ Comments backwardPage[commentNode] }
		}
		err := client.DoWithContext(ctx, commentsPageQuery, pageVariables(node.ID, cursor), &response)
		return response.Node.Comments, err
	})
	if err != nil {
		return nil, err
	}

	var comments []Comment
	for _, comment := range nodes {
		if isRecentHumanComment(comment, since) {
			comments = append(comments, newComment(comment, node))
		}
	}
	return comments, nil
}

func fetchInlineComments(ctx context.Context, client GraphQLClient, node updatedNode, since time.Time) ([]Comment, error) {
	// Threads are ordered by when they started, and an old thread can have a new
	// comment, so every page has to be read.
	threads, err := walkBack(node.ReviewThreads, func(threadNode) bool { return true }, func(cursor string) (backwardPage[threadNode], error) {
		var response struct {
			Node struct{ ReviewThreads backwardPage[threadNode] }
		}
		err := client.DoWithContext(ctx, threadsPageQuery, pageVariables(node.ID, cursor), &response)
		return response.Node.ReviewThreads, err
	})
	if err != nil {
		return nil, err
	}

	var comments []Comment
	for _, thread := range threads {
		threadComments, err := fetchThreadComments(ctx, client, thread, since)
		if err != nil {
			return nil, err
		}
		for _, comment := range threadComments {
			if !isRecentHumanComment(comment.commentNode, since) {
				continue
			}
			parsed := newComment(comment.commentNode, node)
			if comment.PullRequestReview != nil {
				parsed.ReviewURL = comment.PullRequestReview.URL
			}
			comments = append(comments, parsed)
		}
	}
	return comments, nil
}

func fetchThreadComments(ctx context.Context, client GraphQLClient, thread threadNode, since time.Time) ([]threadCommentNode, error) {
	return walkBack(thread.Comments, func(oldest threadCommentNode) bool { return !oldest.CreatedAt.Before(since) }, func(cursor string) (backwardPage[threadCommentNode], error) {
		var response struct {
			Node struct {
				Comments backwardPage[threadCommentNode]
			}
		}
		err := client.DoWithContext(ctx, threadCommentsPageQuery, pageVariables(thread.ID, cursor), &response)
		return response.Node.Comments, err
	})
}

func pageVariables(id, cursor string) map[string]interface{} {
	return map[string]interface{}{"id": id, "cursor": cursor}
}

func isRecentHumanComment(comment commentNode, since time.Time) bool {
	return !comment.CreatedAt.Before(since) && !comment.Author.isBot()
}

func newComment(comment commentNode, parent updatedNode) Comment {
	return Comment{
		Author:        comment.Author.login(),
		Excerpt:       excerpt(comment.BodyText),
		CreatedAt:     comment.CreatedAt,
		URL:           comment.URL,
		ParentNumber:  parent.Number,
		ParentAuthor:  parent.Author.login(),
		OnPullRequest: parent.Typename == "PullRequest",
	}
}

// excerpt returns body on one line, cut by runes so a character is never split.
func excerpt(body string) string {
	runes := []rune(strings.Join(strings.Fields(body), " "))
	if len(runes) <= excerptLength {
		return string(runes)
	}
	return string(runes[:excerptLength]) + "…"
}
