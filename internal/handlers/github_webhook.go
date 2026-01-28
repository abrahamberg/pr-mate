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

	if h.webhookProc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "webhook processor not configured"})
		return
	}

	if err := h.webhookProc.Enqueue(req.Context(), eventType, payload, deliveryID); err != nil {
		log.Printf("webhook enqueue failed event=%s delivery=%s err=%v", eventType, deliveryID, err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "webhook queue full"})
		return
	}

	c.Status(http.StatusAccepted)
}
