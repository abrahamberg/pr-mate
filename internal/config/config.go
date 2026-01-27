package config

import (
	"os"
	"time"
)

// Config holds application configuration
type Config struct {
	Port            string
	GinMode         string
	CopilotModel    string
	ShutdownTimeout time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
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

	return &Config{
		Port:            port,
		GinMode:         ginMode,
		CopilotModel:    copilotModel,
		ShutdownTimeout: 10 * time.Second,
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    15 * time.Second,
		IdleTimeout:     60 * time.Second,
	}
}
