package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstructionsReader_ReadInstructions(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .github directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".github"), 0755); err != nil {
		t.Fatalf("create .github: %v", err)
	}

	// Create copilot-instructions.md
	content := `# Project Guidelines

## Code Style

- Use descriptive names
- Keep functions short
- Wrap errors with context

## Testing

- Write unit tests
- Use table-driven tests
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".github/copilot-instructions.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	reader := NewInstructionsReader()
	instructions, err := reader.ReadInstructions(tmpDir)
	if err != nil {
		t.Fatalf("read instructions: %v", err)
	}

	if len(instructions) != 1 {
		t.Errorf("expected 1 instruction file, got %d", len(instructions))
	}

	if instructions[0].Type != "copilot" {
		t.Errorf("expected type 'copilot', got '%s'", instructions[0].Type)
	}

	if len(instructions[0].Sections) < 2 {
		t.Errorf("expected at least 2 sections, got %d", len(instructions[0].Sections))
	}
}

func TestInstructionsReader_ExtractRulesFromInstructions(t *testing.T) {
	instructions := []InstructionFile{
		{
			Type: "copilot",
			Sections: []InstructionSection{
				{
					Title:   "Code Style Rules",
					Content: "- Use descriptive variable names\n- Keep functions under 50 lines\n- Short",
					Level:   2,
				},
				{
					Title:   "Other Section",
					Content: "Some content without rules",
					Level:   2,
				},
			},
		},
	}

	reader := NewInstructionsReader()
	rules := reader.ExtractRulesFromInstructions(instructions)

	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d: %v", len(rules), rules)
	}
}

func TestInstructionsReader_HasScanDirective(t *testing.T) {
	reader := NewInstructionsReader()

	tests := []struct {
		content  string
		expected bool
	}{
		{"<!-- PRMate\n@scan\nowner/repo\n-->", true},
		{"@scan", true},
		{"Some regular content", false},
		{"@scanned", false}, // Already processed
	}

	for _, tt := range tests {
		result := reader.HasScanDirective(tt.content)
		if result != tt.expected {
			t.Errorf("HasScanDirective(%q) = %v, want %v", tt.content, result, tt.expected)
		}
	}
}

func TestInstructionsReader_HasPRMateDirective(t *testing.T) {
	reader := NewInstructionsReader()

	tests := []struct {
		content  string
		expected bool
	}{
		{"@prmate please review", true},
		{"@prmate", true},
		{"Some regular comment", false},
	}

	for _, tt := range tests {
		result := reader.HasPRMateDirective(tt.content)
		if result != tt.expected {
			t.Errorf("HasPRMateDirective(%q) = %v, want %v", tt.content, result, tt.expected)
		}
	}
}

func TestInstructionsReader_ParseScanDirective(t *testing.T) {
	reader := NewInstructionsReader()

	content := `# PRMate Context

<!-- PRMate
@scan
owner/repo1
github.com/owner/repo2
https://github.com/owner/repo3
-->

Some other content
`

	repos := reader.ParseScanDirective(content)

	if len(repos) != 3 {
		t.Errorf("expected 3 repos, got %d: %v", len(repos), repos)
	}

	expected := []string{"owner/repo1", "github.com/owner/repo2", "https://github.com/owner/repo3"}
	for i, exp := range expected {
		if i >= len(repos) || repos[i] != exp {
			t.Errorf("repo %d: expected %s, got %s", i, exp, repos[i])
		}
	}
}

func TestInstructionsReader_RemoveScanDirective(t *testing.T) {
	reader := NewInstructionsReader()

	content := "Some content\n@scan\nMore content"
	result := reader.RemoveScanDirective(content)

	if result != "Some content\n@scanned\nMore content" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestInstructionsReader_ParseMarkdownSections(t *testing.T) {
	reader := NewInstructionsReader()

	content := `# Main Title

Some intro text

## Section One

Content for section one

### Subsection

Subsection content

## Section Two

Content for section two
`

	sections := reader.parseMarkdownSections(content)

	if len(sections) != 4 {
		t.Errorf("expected 4 sections, got %d", len(sections))
	}

	if sections[0].Title != "Main Title" || sections[0].Level != 1 {
		t.Errorf("first section: got title=%s level=%d", sections[0].Title, sections[0].Level)
	}

	if sections[1].Title != "Section One" || sections[1].Level != 2 {
		t.Errorf("second section: got title=%s level=%d", sections[1].Title, sections[1].Level)
	}
}
