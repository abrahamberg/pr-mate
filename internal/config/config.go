package config

import (
	"os"
	"time"
)

// Config holds application configuration
type Config struct {
	Port             string
	GinMode          string
	CopilotModel     string
	GitHubToken      string
	WebhookSecret    string
	WorkBaseDir      string
	WebhookQueueSize int
	WebhookWorkers   int
	ShutdownTimeout  time.Duration
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	IdleTimeout      time.Duration
}

// Load loads configuration from environment variables
func Load() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		ginMode = "debug"
	}

	copilotModel := os.Getenv("COPILOT_MODEL")
	if copilotModel == "" {
		copilotModel = "gpt-5-mini"
	}

	webhookSecret := os.Getenv("WEBHOOK_SECRET")
	githubToken := os.Getenv("GITHUB_TOKEN")

	workBaseDir := os.Getenv("PR_WORK_BASE_DIR")
	if workBaseDir == "" {
		workBaseDir = "/tmp/prmate"
	}

	webhookQueueSize := 100
	if v := os.Getenv("WEBHOOK_QUEUE_SIZE"); v != "" {
		if parsed, err := parsePositiveInt(v); err == nil {
			webhookQueueSize = parsed
		}
	}

	webhookWorkers := 1
	if v := os.Getenv("WEBHOOK_WORKERS"); v != "" {
		if parsed, err := parsePositiveInt(v); err == nil {
			webhookWorkers = parsed
		}
	}

	return &Config{
		Port:             port,
		GinMode:          ginMode,
		CopilotModel:     copilotModel,
		GitHubToken:      githubToken,
		WebhookSecret:    webhookSecret,
		WorkBaseDir:      workBaseDir,
		WebhookQueueSize: webhookQueueSize,
		WebhookWorkers:   webhookWorkers,
		ShutdownTimeout:  10 * time.Second,
		ReadTimeout:      15 * time.Second,
		WriteTimeout:     15 * time.Second,
		IdleTimeout:      60 * time.Second,
	}
}

func parsePositiveInt(s string) (int, error) {
	// tiny helper to avoid pulling in extra config libs
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, os.ErrInvalid
		}
		n = n*10 + int(r-'0')
	}
	if n <= 0 {
		return 0, os.ErrInvalid
	}
	return n, nil
}
