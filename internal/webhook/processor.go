package webhook

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/google/go-github/v82/github"

	ghclient "prmate/internal/github"
	"prmate/internal/scan"
)

type PRWorkspace interface {
	EnsurePRDir(ctx context.Context, repoFullName string, prNumber int) (string, error)
	DeletePRDir(ctx context.Context, repoFullName string, prNumber int) error
}

// ScanService defines the interface for codebase scanning
type ScanService interface {
	ProcessScan(ctx context.Context, req scan.ScanRequest) (*scan.ScanResult, error)
	CheckForScanDirective(ctx context.Context, owner, repo, branch string) (bool, []string, error)
	CheckForPRMateDirective(content string) bool
}

type Processor struct {
	prWorkspace  PRWorkspace
	scanService  ScanService
	githubClient *ghclient.Client
}

func NewProcessor(prWorkspace PRWorkspace, scanService ScanService, githubClient *ghclient.Client) *Processor {
	return &Processor{
		prWorkspace:  prWorkspace,
		scanService:  scanService,
		githubClient: githubClient,
	}
}

func (p *Processor) Process(ctx context.Context, eventType string, payload []byte, deliveryID string) error {
	_ = deliveryID
	if p.prWorkspace == nil {
		return fmt.Errorf("pr workspace not configured")
	}

	event, err := github.ParseWebHook(eventType, payload)
	if err != nil {
		return fmt.Errorf("parse webhook event: %w", err)
	}

	switch e := event.(type) {
	case *github.PingEvent:
		return nil
	case *github.PullRequestEvent:
		return p.handlePullRequest(ctx, e)
	case *github.IssueCommentEvent:
		return p.handleIssueComment(ctx, e)
	default:
		return nil
	}
}

func (p *Processor) handlePullRequest(ctx context.Context, e *github.PullRequestEvent) error {
	action := strings.ToLower(e.GetAction())
	repoFullName := e.GetRepo().GetFullName()
	prNumber := e.GetPullRequest().GetNumber()
	branch := e.GetPullRequest().GetHead().GetRef()

	owner, repo, err := ghclient.ParseRepoFullName(repoFullName)
	if err != nil {
		return fmt.Errorf("parse repo name: %w", err)
	}

	switch action {
	case "opened", "reopened", "synchronize":
		_, err := p.prWorkspace.EnsurePRDir(ctx, repoFullName, prNumber)
		if err != nil {
			return fmt.Errorf("ensure pr workspace: %w", err)
		}

		// Check for @scan directive in .prmate.md
		if p.scanService != nil {
			if err := p.checkAndProcessScan(ctx, owner, repo, prNumber, branch); err != nil {
				log.Printf("scan processing failed: %v", err)
				// Don't fail the webhook, just log
			}
		}

		return nil
	case "closed":
		if err := p.prWorkspace.DeletePRDir(ctx, repoFullName, prNumber); err != nil {
			return fmt.Errorf("delete pr workspace: %w", err)
		}
		return nil
	default:
		return nil
	}
}

func (p *Processor) checkAndProcessScan(ctx context.Context, owner, repo string, prNumber int, branch string) error {
	hasScan, externalRepos, err := p.scanService.CheckForScanDirective(ctx, owner, repo, branch)
	if err != nil {
		return fmt.Errorf("check scan directive: %w", err)
	}

	if !hasScan {
		return nil
	}

	log.Printf("Found @scan directive in %s/%s PR #%d, external repos: %v", owner, repo, prNumber, externalRepos)

	// Process the scan
	req := scan.ScanRequest{
		Owner:         owner,
		Repo:          repo,
		PRNumber:      prNumber,
		Branch:        branch,
		ExternalRepos: externalRepos,
	}

	result, err := p.scanService.ProcessScan(ctx, req)
	if err != nil {
		// Comment on PR about the failure
		if p.githubClient != nil {
			_ = p.githubClient.CreatePRComment(ctx, owner, repo, prNumber,
				fmt.Sprintf("❌ PRMate scan failed: %v", err))
		}
		return fmt.Errorf("process scan: %w", err)
	}

	// Comment on PR about success
	if p.githubClient != nil {
		_ = p.githubClient.CreatePRComment(ctx, owner, repo, prNumber,
			"✅ PRMate scan completed. `.prmate.md` has been updated with codebase context.")
	}

	log.Printf("Scan completed for %s/%s PR #%d, temp file: %s", owner, repo, prNumber, result.TempFilePath)

	return nil
}

// handleIssueComment processes issue/PR comment events for @prmate directive
func (p *Processor) handleIssueComment(ctx context.Context, e *github.IssueCommentEvent) error {
	// Only handle PR comments (issues with pull_request field)
	if e.GetIssue().GetPullRequestLinks() == nil {
		return nil
	}

	action := strings.ToLower(e.GetAction())
	if action != "created" {
		return nil
	}

	body := e.GetComment().GetBody()
	if !p.scanService.CheckForPRMateDirective(body) {
		return nil
	}

	repoFullName := e.GetRepo().GetFullName()
	prNumber := e.GetIssue().GetNumber()

	owner, repo, err := ghclient.ParseRepoFullName(repoFullName)
	if err != nil {
		return fmt.Errorf("parse repo name: %w", err)
	}

	// Get PR branch
	branch, err := p.githubClient.GetPRBranch(ctx, owner, repo, prNumber)
	if err != nil {
		return fmt.Errorf("get pr branch: %w", err)
	}

	log.Printf("Found @prmate directive in comment on %s/%s PR #%d", owner, repo, prNumber)

	// Check for @scan directive and process
	return p.checkAndProcessScan(ctx, owner, repo, prNumber, branch)
}
