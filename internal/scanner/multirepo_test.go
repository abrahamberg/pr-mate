package scanner

import (
	"testing"
)

func TestNormalizeRepoAddress(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"owner/repo", "github.com/owner/repo"},
		{"github.com/owner/repo", "github.com/owner/repo"},
		{"https://github.com/owner/repo", "github.com/owner/repo"},
		{"http://github.com/owner/repo", "github.com/owner/repo"},
		{"github.com/owner/repo.git", "github.com/owner/repo"},
		{"https://github.com/owner/repo.git", "github.com/owner/repo"},
	}

	for _, tt := range tests {
		result := normalizeRepoAddress(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeRepoAddress(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"github.com/owner/repo", "owner_repo"},
		{"owner/repo", "owner_repo"},
		{"repo", "repo"},
	}

	for _, tt := range tests {
		result := extractRepoName(tt.input)
		if result != tt.expected {
			t.Errorf("extractRepoName(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestMergeAnalysisResults(t *testing.T) {
	current := &AnalysisResult{
		FolderNaming: NamingCamelCase,
		FileNaming:   NamingSnakeCase,
		Abstractions: []AbstractionInfo{
			{Name: "Service", Suffix: "Service"},
		},
		NamingPatterns: []PatternMatch{
			{Pattern: "*_test", Count: 5},
		},
	}

	external := &AnalysisResult{
		FolderNaming: NamingKebabCase,
		FileNaming:   NamingPascalCase,
		Abstractions: []AbstractionInfo{
			{Name: "Service", Suffix: "Service"}, // Duplicate
			{Name: "Repository", Suffix: "Repository"},
		},
		NamingPatterns: []PatternMatch{
			{Pattern: "*_test", Count: 10},  // Duplicate
			{Pattern: "*Handler", Count: 3},
		},
	}

	merged := MergeAnalysisResults(current, []*AnalysisResult{external})

	// Current repo naming should be preserved
	if merged.FolderNaming != NamingCamelCase {
		t.Errorf("expected FolderNaming=camelCase, got %v", merged.FolderNaming)
	}

	if merged.FileNaming != NamingSnakeCase {
		t.Errorf("expected FileNaming=snake_case, got %v", merged.FileNaming)
	}

	// Should have Service (from current) and Repository (from external)
	if len(merged.Abstractions) != 2 {
		t.Errorf("expected 2 abstractions, got %d", len(merged.Abstractions))
	}

	hasRepository := false
	for _, abs := range merged.Abstractions {
		if abs.Name == "Repository" {
			hasRepository = true
		}
	}
	if !hasRepository {
		t.Error("expected Repository abstraction from external")
	}

	// Should have *_test (from current) and *Handler (from external)
	if len(merged.NamingPatterns) != 2 {
		t.Errorf("expected 2 naming patterns, got %d", len(merged.NamingPatterns))
	}
}

func TestMergeAnalysisResults_NilExternals(t *testing.T) {
	current := &AnalysisResult{
		FolderNaming: NamingCamelCase,
	}

	merged := MergeAnalysisResults(current, []*AnalysisResult{nil, nil})

	if merged.FolderNaming != NamingCamelCase {
		t.Errorf("expected FolderNaming preserved with nil externals")
	}
}
