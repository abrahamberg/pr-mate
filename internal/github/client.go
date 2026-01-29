package github

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/google/go-github/v82/github"
)

// Client provides GitHub API operations
type Client struct {
	client *github.Client
	token  string
}

// NewClient creates a new GitHub API client
func NewClient(token string) *Client {
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	httpClient := &http.Client{
		Transport: &tokenTransport{token: token},
	}

	return &Client{
		client: github.NewClient(httpClient),
		token:  token,
	}
}

type tokenTransport struct {
	token string
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "token "+t.token)
	return http.DefaultTransport.RoundTrip(req)
}

// GetToken returns the configured token (for repo cloning)
func (c *Client) GetToken() string {
	return c.token
}

// PRFile represents a file changed in a PR
type PRFile struct {
	Filename  string
	Status    string // added, removed, modified, renamed
	Additions int
	Deletions int
	Patch     string
}

// GetPRFiles fetches files changed in a PR
func (c *Client) GetPRFiles(ctx context.Context, owner, repo string, prNumber int) ([]PRFile, error) {
	opts := &github.ListOptions{PerPage: 100}
	var allFiles []PRFile

	for {
		files, resp, err := c.client.PullRequests.ListFiles(ctx, owner, repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("list pr files: %w", err)
		}

		for _, f := range files {
			allFiles = append(allFiles, PRFile{
				Filename:  f.GetFilename(),
				Status:    f.GetStatus(),
				Additions: f.GetAdditions(),
				Deletions: f.GetDeletions(),
				Patch:     f.GetPatch(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allFiles, nil
}

// GetPRBranch returns the branch name of a PR
func (c *Client) GetPRBranch(ctx context.Context, owner, repo string, prNumber int) (string, error) {
	pr, _, err := c.client.PullRequests.Get(ctx, owner, repo, prNumber)
	if err != nil {
		return "", fmt.Errorf("get pr: %w", err)
	}
	return pr.GetHead().GetRef(), nil
}

// GetFileContent fetches file content from a repo
func (c *Client) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	content, _, _, err := c.client.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{
		Ref: ref,
	})
	if err != nil {
		return "", fmt.Errorf("get file content: %w", err)
	}

	if content == nil {
		return "", fmt.Errorf("file not found: %s", path)
	}

	decoded, err := content.GetContent()
	if err != nil {
		return "", fmt.Errorf("decode content: %w", err)
	}

	return decoded, nil
}

// CreatePRComment creates a comment on a PR
func (c *Client) CreatePRComment(ctx context.Context, owner, repo string, prNumber int, body string) error {
	_, _, err := c.client.Issues.CreateComment(ctx, owner, repo, prNumber, &github.IssueComment{
		Body: github.Ptr(body),
	})
	if err != nil {
		return fmt.Errorf("create pr comment: %w", err)
	}
	return nil
}

// CloneURL returns the authenticated clone URL for a repo
func (c *Client) CloneURL(owner, repo string) string {
	return fmt.Sprintf("https://%s@github.com/%s/%s.git", c.token, owner, repo)
}

// ParseRepoFullName splits "owner/repo" into parts
func ParseRepoFullName(fullName string) (owner, repo string, err error) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo name: %s", fullName)
	}
	return parts[0], parts[1], nil
}

// PullRequest represents essential PR details
type PullRequest struct {
	Number    int
	Title     string
	Body      string
	State     string
	HeadSHA   string
	HeadRef   string
	BaseSHA   string
	BaseRef   string
	Mergeable bool
}

// GetPullRequest fetches full PR details
func (c *Client) GetPullRequest(ctx context.Context, owner, repo string, prNumber int) (*PullRequest, error) {
	pr, _, err := c.client.PullRequests.Get(ctx, owner, repo, prNumber)
	if err != nil {
		return nil, fmt.Errorf("get pull request: %w", err)
	}

	return &PullRequest{
		Number:    pr.GetNumber(),
		Title:     pr.GetTitle(),
		Body:      pr.GetBody(),
		State:     pr.GetState(),
		HeadSHA:   pr.GetHead().GetSHA(),
		HeadRef:   pr.GetHead().GetRef(),
		BaseSHA:   pr.GetBase().GetSHA(),
		BaseRef:   pr.GetBase().GetRef(),
		Mergeable: pr.GetMergeable(),
	}, nil
}

// Commit represents a commit in a PR
type Commit struct {
	SHA     string
	Message string
	Author  string
}

// ListPRCommits lists all commits in a PR
func (c *Client) ListPRCommits(ctx context.Context, owner, repo string, prNumber int) ([]Commit, error) {
	opts := &github.ListOptions{PerPage: 100}
	var allCommits []Commit

	for {
		commits, resp, err := c.client.PullRequests.ListCommits(ctx, owner, repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("list pr commits: %w", err)
		}

		for _, c := range commits {
			allCommits = append(allCommits, Commit{
				SHA:     c.GetSHA(),
				Message: c.GetCommit().GetMessage(),
				Author:  c.GetCommit().GetAuthor().GetName(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allCommits, nil
}

// ReviewComment represents a review comment on a specific line
type ReviewComment struct {
	ID        int64
	Path      string
	Line      int
	Side      string // LEFT or RIGHT
	Body      string
	CommitID  string
	CreatedAt string
}

// ListReviewComments lists all review comments on a PR
func (c *Client) ListReviewComments(ctx context.Context, owner, repo string, prNumber int) ([]ReviewComment, error) {
	opts := &github.PullRequestListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	var allComments []ReviewComment

	for {
		comments, resp, err := c.client.PullRequests.ListComments(ctx, owner, repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("list review comments: %w", err)
		}

		for _, c := range comments {
			allComments = append(allComments, ReviewComment{
				ID:        c.GetID(),
				Path:      c.GetPath(),
				Line:      c.GetLine(),
				Side:      c.GetSide(),
				Body:      c.GetBody(),
				CommitID:  c.GetCommitID(),
				CreatedAt: c.GetCreatedAt().String(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allComments, nil
}

// DraftReviewComment represents a comment to be added in a review
type DraftReviewComment struct {
	Path string
	Line int
	Side string // LEFT or RIGHT (default RIGHT for new file)
	Body string
}

// CreatePullRequestReview creates a review with inline comments
func (c *Client) CreatePullRequestReview(ctx context.Context, owner, repo string, prNumber int, commitID string, event string, body string, comments []DraftReviewComment) error {
	reviewComments := make([]*github.DraftReviewComment, len(comments))
	for i, c := range comments {
		side := c.Side
		if side == "" {
			side = "RIGHT"
		}
		reviewComments[i] = &github.DraftReviewComment{
			Path: github.Ptr(c.Path),
			Line: github.Ptr(c.Line),
			Side: github.Ptr(side),
			Body: github.Ptr(c.Body),
		}
	}

	review := &github.PullRequestReviewRequest{
		CommitID: github.Ptr(commitID),
		Body:     github.Ptr(body),
		Event:    github.Ptr(event), // APPROVE, REQUEST_CHANGES, COMMENT
		Comments: reviewComments,
	}

	_, _, err := c.client.PullRequests.CreateReview(ctx, owner, repo, prNumber, review)
	if err != nil {
		return fmt.Errorf("create pull request review: %w", err)
	}

	return nil
}

// ListPRComments lists all issue-level comments on a PR
func (c *Client) ListPRComments(ctx context.Context, owner, repo string, prNumber int) ([]string, error) {
	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	var bodies []string

	for {
		comments, resp, err := c.client.Issues.ListComments(ctx, owner, repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("list pr comments: %w", err)
		}

		for _, c := range comments {
			bodies = append(bodies, c.GetBody())
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return bodies, nil
}

// ParsePatchHunks parses a patch to extract line number mappings
// Returns a map from new file line number to diff position (for review comments)
type PatchHunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	Lines    []PatchLine
}

type PatchLine struct {
	Type       string // "add", "remove", "context"
	Content    string
	OldLineNo  int
	NewLineNo  int
	DiffPos    int // Position in the diff (1-indexed)
}

// ParsePatch parses a unified diff patch into structured hunks
func ParsePatch(patch string) []PatchHunk {
	if patch == "" {
		return nil
	}

	hunks := make([]PatchHunk, 0)
	lines := strings.Split(patch, "\n")

	var currentHunk *PatchHunk
	diffPos := 0
	oldLine := 0
	newLine := 0

	for _, line := range lines {
		diffPos++

		// Parse hunk header: @@ -oldStart,oldLines +newStart,newLines @@
		if strings.HasPrefix(line, "@@") {
			if currentHunk != nil {
				hunks = append(hunks, *currentHunk)
			}

			hunk := PatchHunk{}
			// Parse the header
			_, err := fmt.Sscanf(line, "@@ -%d,%d +%d,%d @@",
				&hunk.OldStart, &hunk.OldLines, &hunk.NewStart, &hunk.NewLines)
			if err != nil {
				// Try single line format: @@ -1 +1 @@
				_, _ = fmt.Sscanf(line, "@@ -%d +%d @@", &hunk.OldStart, &hunk.NewStart)
				hunk.OldLines = 1
				hunk.NewLines = 1
			}

			oldLine = hunk.OldStart
			newLine = hunk.NewStart
			currentHunk = &hunk
			continue
		}

		if currentHunk == nil {
			continue
		}

		patchLine := PatchLine{
			Content: line,
			DiffPos: diffPos,
		}

		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			patchLine.Type = "add"
			patchLine.NewLineNo = newLine
			newLine++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			patchLine.Type = "remove"
			patchLine.OldLineNo = oldLine
			oldLine++
		} else {
			patchLine.Type = "context"
			patchLine.OldLineNo = oldLine
			patchLine.NewLineNo = newLine
			oldLine++
			newLine++
		}

		currentHunk.Lines = append(currentHunk.Lines, patchLine)
	}

	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}

	return hunks
}

// GetNewLineNumbers returns all new/added line numbers from a patch
func GetNewLineNumbers(patch string) []int {
	hunks := ParsePatch(patch)
	lines := make([]int, 0)

	for _, hunk := range hunks {
		for _, line := range hunk.Lines {
			if line.Type == "add" {
				lines = append(lines, line.NewLineNo)
			}
		}
	}

	return lines
}
