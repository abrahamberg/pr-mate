package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzer_DetectNamingStyle(t *testing.T) {
	tests := []struct {
		name     string
		expected NamingStyle
	}{
		{"camelCase", NamingCamelCase},
		{"PascalCase", NamingPascalCase},
		{"snake_case", NamingSnakeCase},
		{"kebab-case", NamingKebabCase},
		{"", NamingMixed},
	}

	for _, tt := range tests {
		result := detectNamingStyle(tt.name)
		if result != tt.expected {
			t.Errorf("detectNamingStyle(%s) = %v, want %v", tt.name, result, tt.expected)
		}
	}
}

func TestAnalyzer_Analyze(t *testing.T) {
	// Create a temp directory with test files
	tmpDir := t.TempDir()

	// Create folder structure
	dirs := []string{
		"internal/handlers",
		"internal/config",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, d), 0755); err != nil {
			t.Fatalf("create dir: %v", err)
		}
	}

	// Create test files with patterns
	files := map[string]string{
		"internal/handlers/handler.go": `package handlers

type Handler struct{}

func (h *Handler) Health() {}
`,
		"internal/config/config.go": `package config

import "fmt"

type Config struct{}

func Load() (*Config, error) {
	if err := validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return &Config{}, nil
}

func validate() error { return nil }
`,
		"internal/handlers/service.go": `package handlers

type UserService struct{}
type OrderService struct{}
`,
	}
	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	// Scan and analyze
	scanner := NewScanner()
	ctx, err := scanner.Scan(tmpDir)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	analyzer := NewAnalyzer()
	result, err := analyzer.Analyze(ctx)
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}

	// Verify folder convention detected
	if len(result.FolderConventions) == 0 {
		t.Error("expected folder conventions to be detected")
	}

	foundInternal := false
	for _, conv := range result.FolderConventions {
		if conv.Pattern == "internal/{domain}/" {
			foundInternal = true
			break
		}
	}
	if !foundInternal {
		t.Error("expected internal/{domain}/ convention to be detected")
	}

	// Verify error wrapping pattern detected
	foundWrap := false
	for _, pattern := range result.ErrorPatterns {
		if pattern.Style == "wrap" {
			foundWrap = true
			break
		}
	}
	if !foundWrap {
		t.Error("expected error wrapping pattern to be detected")
	}

	// Verify Service abstraction detected
	foundService := false
	for _, abs := range result.Abstractions {
		if abs.Name == "Service" {
			foundService = true
			break
		}
	}
	if !foundService {
		t.Error("expected Service abstraction to be detected")
	}
}

func TestAnalyzer_DominantStyle(t *testing.T) {
	tests := []struct {
		styles   map[NamingStyle]int
		expected NamingStyle
	}{
		{map[NamingStyle]int{NamingCamelCase: 5, NamingSnakeCase: 2}, NamingCamelCase},
		{map[NamingStyle]int{NamingSnakeCase: 10, NamingKebabCase: 3}, NamingSnakeCase},
		{map[NamingStyle]int{}, NamingMixed},
	}

	for i, tt := range tests {
		result := dominantStyle(tt.styles)
		if result != tt.expected {
			t.Errorf("test %d: dominantStyle() = %v, want %v", i, result, tt.expected)
		}
	}
}
