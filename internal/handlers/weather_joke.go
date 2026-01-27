package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type WeatherJokeRequest struct {
	City string `json:"city" binding:"required"`
}

type WeatherJokeResponse struct {
	City    string `json:"city"`
	Weather string `json:"weather"`
	Joke    string `json:"joke"`
}

func (h *Handler) WeatherJoke(c *gin.Context) {
	var req WeatherJokeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "city is required"})
		return
	}

	weatherInfo, err := h.weatherService.GetWeather(req.City)
	if err != nil {
		log.Printf("weather lookup failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get weather"})
		return
	}

	prompt := fmt.Sprintf(
		"The weather in %s is %s with %s conditions. Make a short funny joke about this weather.",
		req.City,
		weatherInfo.Temperature,
		weatherInfo.Condition,
	)

	joke, err := h.copilotService.GenerateText(prompt)
	if err != nil {
		log.Printf("joke generation failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate joke"})
		return
	}

	c.JSON(http.StatusOK, WeatherJokeResponse{
		City:    req.City,
		Weather: fmt.Sprintf("%s, %s", weatherInfo.Temperature, weatherInfo.Condition),
		Joke:    joke,
	})
}
