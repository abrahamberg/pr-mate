package weather

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Result represents weather information for a city
type Result struct {
	City        string `json:"city"`
	Temperature string `json:"temperature"`
	Condition   string `json:"condition"`
}

// Service provides weather-related functionality
type Service struct {
	mu  sync.Mutex
	rng *rand.Rand
}

// NewService creates a new weather service
func NewService() *Service {
	return &Service{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GetWeather retrieves weather information for a city
// In a real app, this would call a weather API
func (s *Service) GetWeather(city string) (Result, error) {
	conditions := []string{"sunny", "cloudy", "rainy", "partly cloudy"}

	s.mu.Lock()
	temp := s.rng.Intn(30) + 50
	condition := conditions[s.rng.Intn(len(conditions))]
	s.mu.Unlock()

	return Result{
		City:        city,
		Temperature: fmt.Sprintf("%d°F", temp),
		Condition:   condition,
	}, nil
}
