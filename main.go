package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"prmate/internal/config"
	"prmate/internal/copilot"
	"prmate/internal/github"
	"prmate/internal/handlers"
	"prmate/internal/llm"
	"prmate/internal/prworkspace"
	"prmate/internal/review"
	"prmate/internal/scan"
	"prmate/internal/server"
	"prmate/internal/weather"
	"prmate/internal/webhook"
)

// LLMService defines the interface for LLM providers used by the application
type LLMService interface {
	GenerateText(prompt string) (string, error)
	Start() error
	Stop() error
}

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize LLM service based on configuration
	var llmSvc LLMService
	switch cfg.LLMProvider {
	case "openai":
		log.Printf("Using OpenAI LLM provider (model: %s)", cfg.OpenAIModel)
		llmSvc = llm.NewOpenAIProvider(llm.OpenAIConfig{
			APIKey:  cfg.OpenAIAPIKey,
			BaseURL: cfg.OpenAIBaseURL,
			Model:   cfg.OpenAIModel,
		})
	default:
		log.Printf("Using Copilot LLM provider (model: %s)", cfg.CopilotModel)
		llmSvc = copilot.NewService(cfg.CopilotModel)
	}

	if err := llmSvc.Start(); err != nil {
		log.Fatalf("Failed to start LLM service: %v", err)
	}
	defer llmSvc.Stop()

	// Initialize GitHub client
	githubClient := github.NewClient(cfg.GitHubToken)

	// Initialize services
	weatherSvc := weather.NewService()
	prWorkspaceMgr := prworkspace.NewManager(cfg.WorkBaseDir)
	scanSvc := scan.NewService(githubClient)
	reviewSvc := review.NewService(githubClient, llmSvc)
	webhookProc := webhook.NewProcessor(prWorkspaceMgr, scanSvc, reviewSvc, githubClient)
	webhookAsync := webhook.NewAsyncProcessor(webhookProc, webhook.AsyncConfig{QueueSize: cfg.WebhookQueueSize, Workers: cfg.WebhookWorkers})

	// Setup HTTP server
	srv := server.NewServer(cfg)
	handler := handlers.NewHandler(llmSvc, weatherSvc, webhookAsync, cfg.WebhookSecret)

	// Register routes
	srv.Router().GET("/health", handler.Health)
	srv.Router().POST("/api/weather-joke", handler.WeatherJoke)
	srv.Router().POST("/webhook", handler.GitHubWebhook)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-quit:
		log.Println("Shutdown signal received")
	case err := <-errCh:
		if err != nil {
			log.Printf("Server error: %v", err)
		}
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	if err := webhookAsync.Stop(ctx); err != nil {
		log.Printf("Webhook processor shutdown error: %v", err)
	}

	log.Println("Server exited")
}
