package webhook

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v82/github"
)

type PRWorkspace interface {
	EnsurePRDir(ctx context.Context, repoFullName string, prNumber int) (string, error)
	DeletePRDir(ctx context.Context, repoFullName string, prNumber int) error
}

type Processor struct {
	prWorkspace PRWorkspace
}

func NewProcessor(prWorkspace PRWorkspace) *Processor {
	return &Processor{prWorkspace: prWorkspace}
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
	default:
		return nil
	}
}

func (p *Processor) handlePullRequest(ctx context.Context, e *github.PullRequestEvent) error {
	action := strings.ToLower(e.GetAction())
	repoFullName := e.GetRepo().GetFullName()
	prNumber := e.GetPullRequest().GetNumber()

	switch action {
	case "opened", "reopened", "synchronize":
		_, err := p.prWorkspace.EnsurePRDir(ctx, repoFullName, prNumber)
		if err != nil {
			return fmt.Errorf("ensure pr workspace: %w", err)
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
