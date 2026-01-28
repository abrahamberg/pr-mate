package handlers

import (
	"prmate/internal/weather"
)

type JokeGenerator interface {
	GenerateText(prompt string) (string, error)
}

type WeatherGetter interface {
	GetWeather(city string) (weather.Result, error)
}

// Handler manages HTTP request handlers
type Handler struct {
	copilotService JokeGenerator
	weatherService WeatherGetter
	webhookSecret  string
}

// NewHandler creates a new handler instance
func NewHandler(copilotSvc JokeGenerator, weatherSvc WeatherGetter, webhookSecret string) *Handler {
	return &Handler{
		copilotService: copilotSvc,
		weatherService: weatherSvc,
		webhookSecret:  webhookSecret,
	}
}
