package handlers

import (
	"context"
	"prmate/internal/weather"
)

type JokeGenerator interface {
	GenerateText(prompt string) (string, error)
}

type WeatherGetter interface {
	GetWeather(city string) (weather.Result, error)
}

type WebhookProcessor interface {
	Enqueue(ctx context.Context, eventType string, payload []byte, deliveryID string) error
}

// Handler manages HTTP request handlers
type Handler struct {
	copilotService JokeGenerator
	weatherService WeatherGetter
	webhookProc    WebhookProcessor
	webhookSecret  string
}

// NewHandler creates a new handler instance
func NewHandler(copilotSvc JokeGenerator, weatherSvc WeatherGetter, webhookProc WebhookProcessor, webhookSecret string) *Handler {
	return &Handler{
		copilotService: copilotSvc,
		weatherService: weatherSvc,
		webhookProc:    webhookProc,
		webhookSecret:  webhookSecret,
	}
}
