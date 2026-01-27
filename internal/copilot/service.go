package copilot

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	copilot "github.com/github/copilot-sdk/go"
)

// Service manages Copilot SDK client lifecycle
type Service struct {
	client  *copilot.Client
	model   string
	mu      sync.Mutex
	wg      sync.WaitGroup
	started bool
}

// NewService creates a new Copilot service
func NewService(model string) *Service {
	if model == "" {
		model = "gpt-5-mini"
	}
	return &Service{
		client: copilot.NewClient(nil),
		model:  model,
	}
}

// Start initializes the Copilot client
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}

	if err := s.client.Start(); err != nil {
		return fmt.Errorf("failed to start copilot client: %w", err)
	}

	s.started = true
	return nil
}

// Stop cleanly shuts down the Copilot client
func (s *Service) Stop() error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}

	s.started = false
	s.mu.Unlock()

	s.wg.Wait()

	s.mu.Lock()
	s.client.Stop()
	s.mu.Unlock()
	return nil
}

func (s *Service) createSession() (*copilot.Session, error) {
	session, err := s.client.CreateSession(&copilot.SessionConfig{
		Model:     s.model,
		Streaming: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

// GenerateText generates text from a prompt.
// This is the API HTTP handlers should use (no Copilot SDK types leak outside this package).
func (s *Service) GenerateText(prompt string) (string, error) {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return "", fmt.Errorf("copilot service not started")
	}
	s.wg.Add(1)
	s.mu.Unlock()
	defer s.wg.Done()

	session, err := s.createSession()
	if err != nil {
		return "", err
	}

	var responseMu sync.Mutex
	var responseBuffer bytes.Buffer
	session.On(func(event copilot.SessionEvent) {
		if event.Type == "assistant.message_delta" && event.Data.DeltaContent != nil {
			responseMu.Lock()
			responseBuffer.WriteString(*event.Data.DeltaContent)
			responseMu.Unlock()
		}
	})

	_, err = session.SendAndWait(copilot.MessageOptions{Prompt: prompt}, 0)
	if err != nil {
		return "", fmt.Errorf("failed to send prompt: %w", err)
	}

	responseMu.Lock()
	out := strings.TrimSpace(responseBuffer.String())
	responseMu.Unlock()

	return out, nil
}
