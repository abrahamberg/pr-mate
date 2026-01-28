package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v82/github"
)

func (h *Handler) GitHubWebhook(c *gin.Context) {
	req := c.Request
	// GitHub provides the event name in the X-GitHub-Event header.
	// go-github's github.WebHookType(req) reads the same header.
	eventType := c.GetHeader("X-GitHub-Event")
	deliveryID := c.GetHeader("X-GitHub-Delivery")
	if eventType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing X-GitHub-Event header"})
		return
	}

	secret := []byte(h.webhookSecret)
	if len(secret) == 0 {
		secret = nil
	}

	payload, err := github.ValidatePayload(req, secret)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook payload or signature", "details": err.Error()})
		return
	}

	event, err := github.ParseWebHook(eventType, payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported webhook event", "event_type": eventType, "details": err.Error()})
		return
	}

	summary := gin.H{"event_type": eventType, "delivery_id": deliveryID}

	switch e := event.(type) {
	case *github.PingEvent:
		log.Printf("github webhook ping delivery=%s", deliveryID)
		if e.GetHookID() != 0 {
			summary["hook_id"] = e.GetHookID()
		}
	case *github.PullRequestEvent:
		summary["action"] = e.GetAction()
		summary["repo"] = e.GetRepo().GetFullName()
		summary["pr_number"] = e.GetPullRequest().GetNumber()
	case *github.IssuesEvent:
		summary["action"] = e.GetAction()
		summary["repo"] = e.GetRepo().GetFullName()
		summary["issue_number"] = e.GetIssue().GetNumber()
	case *github.PushEvent:
		summary["repo"] = e.GetRepo().GetName()
		summary["ref"] = e.GetRef()
		summary["commits"] = len(e.Commits)
	default:
		log.Printf("github webhook event=%s delivery=%s", eventType, deliveryID)
	}

	c.JSON(http.StatusOK, summary)
}
