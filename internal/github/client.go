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
