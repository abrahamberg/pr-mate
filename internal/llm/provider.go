// Package llm defines interfaces for LLM providers.
// This abstraction allows switching between different LLM backends
// (Copilot, OpenAI, etc.) without changing consumer code.
//
// Current implementation: internal/copilot uses the Copilot SDK directly.
// Future: implement these interfaces for additional providers.
package llm

import "context"

// TextGenerator provides basic text generation capability.
// This is the minimal interface that any LLM provider must implement.
type TextGenerator interface {
	GenerateText(ctx context.Context, prompt string) (string, error)
}

// Message represents a chat message for multi-turn conversations
type Message struct {
	Role    string // "system", "user", "assistant"
	Content string
}

// ChatCompleter extends TextGenerator with multi-turn conversation support
type ChatCompleter interface {
	TextGenerator
	Chat(ctx context.Context, messages []Message) (string, error)
}

// StreamHandler receives streaming chunks from the LLM
type StreamHandler func(chunk string) error

// StreamingCompleter adds streaming support to ChatCompleter
type StreamingCompleter interface {
	ChatCompleter
	ChatStream(ctx context.Context, messages []Message, handler StreamHandler) error
}

// LLMService is the full service interface with lifecycle management
type LLMService interface {
	TextGenerator
	Start() error
	Stop() error
}
