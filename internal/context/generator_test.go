package context

import (
	"strings"
	"testing"

	"prmate/internal/scanner"
)

func TestGenerator_Generate(t *testing.T) {
	result := &scanner.MultiRepoResult{
		CurrentRepo: &scanner.CodebaseContext{
			RepoName:        "test-repo",
			RootPath:        "/tmp/test-repo",
			TopLevelFolders: []string{"internal", "pkg", "cmd"},
			Extensions:      map[string]int{".go": 20, ".md": 3},
			Files:           make([]scanner.FileInfo, 0),
		},
		CurrentAnalysis: &scanner.AnalysisResult{
			FolderNaming: scanner.NamingCamelCase,
			FileNaming:   scanner.NamingSnakeCase,
			FolderConventions: []scanner.FolderConvention{
				{Pattern: "internal/{domain}/", Purpose: "Private code", Examples: []string{"handlers", "config"}},
			},
			Abstractions: []scanner.AbstractionInfo{
				{Name: "Service", Suffix: "Service", Locations: []string{"/a.go", "/b.go"}},
				{Name: "Interface", IsInterface: true, Locations: []string{"/c.go"}},
			},
			ErrorPatterns: []scanner.ErrorPattern{
				{Style: "wrap", Count: 15},
			},
			TestConventions: scanner.TestConvention{
				Colocated:  true,
				TestSuffix: "_test.go",
			},
		},
		ExternalRepos: []scanner.ExternalRepoData{
			{
				Source: scanner.RepoSource{
					Address:   "owner/other-repo",
					HasPRMate: true,
				},
			},
		},
		MergedRules: []string{
			"Use descriptive names",
			"Wrap errors with context",
		},
	}

	generator := NewGenerator()
	content := generator.Generate(result)

	// Verify key sections are present
	expectedSections := []string{
		"# PRMate Context",
		"## Folder Structure",
		"## Naming Conventions",
		"## Abstractions",
		"## Error Handling",
		"## Test Conventions",
		"## Senior Developer Review Checklist",
		"## Learned Rules",
		"## Sources",
	}

	for _, section := range expectedSections {
		if !strings.Contains(content, section) {
			t.Errorf("missing section: %s", section)
		}
	}

	// Verify specific content
	if !strings.Contains(content, "internal/{domain}/") {
		t.Error("missing folder convention pattern")
	}

	if !strings.Contains(content, "camelCase") {
		t.Error("missing folder naming style")
	}

	if !strings.Contains(content, "snake_case") {
		t.Error("missing file naming style")
	}

	if !strings.Contains(content, "Service") {
		t.Error("missing Service abstraction")
	}

	if !strings.Contains(content, "Error wrapping") {
		t.Error("missing error wrapping pattern")
	}

	if !strings.Contains(content, "Use descriptive names") {
		t.Error("missing learned rule")
	}

	if !strings.Contains(content, "test-repo") {
		t.Error("missing current repo name")
	}

	if !strings.Contains(content, "owner/other-repo") {
		t.Error("missing external repo")
	}
}

func TestGenerator_WriteFolderStructure(t *testing.T) {
	ctx := &scanner.CodebaseContext{
		TopLevelFolders: []string{"internal", "pkg"},
		Extensions:      map[string]int{".go": 10, ".md": 2},
	}
	analysis := &scanner.AnalysisResult{
		FolderConventions: []scanner.FolderConvention{
			{Pattern: "internal/{domain}/", Purpose: "Private code", Examples: []string{"handlers"}},
		},
	}

	generator := NewGenerator()
	var sb strings.Builder
	generator.writeFolderStructure(&sb, ctx, analysis)

	content := sb.String()

	if !strings.Contains(content, "internal/{domain}/") {
		t.Error("missing folder convention")
	}

	if !strings.Contains(content, "`internal/`") {
		t.Error("missing top-level folder")
	}

	if !strings.Contains(content, ".go") {
		t.Error("missing file extension")
	}
}

func TestGenerator_WriteSeniorDevChecklist(t *testing.T) {
	generator := NewGenerator()
	var sb strings.Builder
	generator.writeSeniorDevChecklist(&sb)

	content := sb.String()

	expectedItems := []string{
		"File locations",
		"Abstraction usage",
		"Naming consistency",
		"Error handling",
		"Test coverage",
		"Security patterns",
		"Documentation",
		"Dependency injection",
	}

	for _, item := range expectedItems {
		if !strings.Contains(content, item) {
			t.Errorf("missing checklist item: %s", item)
		}
	}

	// Verify checkbox format
	if !strings.Contains(content, "- [ ]") {
		t.Error("missing checkbox format")
	}
}

func TestGenerator_WriteLearnedRules_Deduplication(t *testing.T) {
	generator := NewGenerator()
	var sb strings.Builder

	rules := []string{
		"Use descriptive names",
		"use descriptive names", // Duplicate (different case)
		"Wrap errors",
		"  Wrap errors  ", // Duplicate (whitespace)
		"Another rule",
	}

	generator.writeLearnedRules(&sb, rules)
	content := sb.String()

	// Count occurrences of each rule
	count := strings.Count(content, "descriptive names")
	if count != 1 {
		t.Errorf("expected 1 occurrence of 'descriptive names', got %d", count)
	}
}
