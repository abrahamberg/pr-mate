package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

// InstructionFile represents a parsed instruction file
type InstructionFile struct {
	Path     string
	Type     string   // "copilot", "cursor", "prmate"
	Content  string
	Sections []InstructionSection
}

// InstructionSection represents a section in an instruction file
type InstructionSection struct {
	Title   string
	Content string
	Level   int // heading level (1-6)
}

// InstructionsReader reads and parses instruction files from a repository
type InstructionsReader struct{}

// NewInstructionsReader creates a new instructions reader
func NewInstructionsReader() *InstructionsReader {
	return &InstructionsReader{}
}

// KnownInstructionFiles lists paths to check for instruction files
var KnownInstructionFiles = []struct {
	Path string
	Type string
}{
	{".github/copilot-instructions.md", "copilot"},
	{".cursorrules", "cursor"},
	{".cursor/rules", "cursor"},
	{".prmate.md", "prmate"},
	{"CONTRIBUTING.md", "contributing"},
	{"docs/CONTRIBUTING.md", "contributing"},
	{".github/CONTRIBUTING.md", "contributing"},
}

// ReadInstructions reads all instruction files from a repository
func (r *InstructionsReader) ReadInstructions(repoPath string) ([]InstructionFile, error) {
	instructions := make([]InstructionFile, 0)

	for _, known := range KnownInstructionFiles {
		fullPath := filepath.Join(repoPath, known.Path)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue // File doesn't exist, skip
		}

		inst := InstructionFile{
			Path:    fullPath,
			Type:    known.Type,
			Content: string(content),
		}

		// Parse sections from markdown files
		if strings.HasSuffix(known.Path, ".md") {
			inst.Sections = r.parseMarkdownSections(string(content))
		} else {
			// For non-markdown files, treat entire content as one section
			inst.Sections = []InstructionSection{
				{Title: "Rules", Content: string(content), Level: 1},
			}
		}

		instructions = append(instructions, inst)
	}

	return instructions, nil
}

// ReadPRMateContext reads the .prmate.md file specifically
func (r *InstructionsReader) ReadPRMateContext(repoPath string) (*InstructionFile, error) {
	fullPath := filepath.Join(repoPath, ".prmate.md")
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	inst := &InstructionFile{
		Path:     fullPath,
		Type:     "prmate",
		Content:  string(content),
		Sections: r.parseMarkdownSections(string(content)),
	}

	return inst, nil
}

// parseMarkdownSections extracts sections from a markdown file
func (r *InstructionsReader) parseMarkdownSections(content string) []InstructionSection {
	sections := make([]InstructionSection, 0)
	lines := strings.Split(content, "\n")

	var currentSection *InstructionSection
	var contentBuilder strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for heading
		if strings.HasPrefix(trimmed, "#") {
			// Save previous section
			if currentSection != nil {
				currentSection.Content = strings.TrimSpace(contentBuilder.String())
				sections = append(sections, *currentSection)
				contentBuilder.Reset()
			}

			// Determine heading level
			level := 0
			for _, c := range trimmed {
				if c == '#' {
					level++
				} else {
					break
				}
			}

			title := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			currentSection = &InstructionSection{
				Title: title,
				Level: level,
			}
		} else if currentSection != nil {
			contentBuilder.WriteString(line)
			contentBuilder.WriteString("\n")
		}
	}

	// Save last section
	if currentSection != nil {
		currentSection.Content = strings.TrimSpace(contentBuilder.String())
		sections = append(sections, *currentSection)
	}

	return sections
}

// ExtractRulesFromInstructions extracts actionable rules from instruction files
func (r *InstructionsReader) ExtractRulesFromInstructions(instructions []InstructionFile) []string {
	rules := make([]string, 0)

	for _, inst := range instructions {
		for _, section := range inst.Sections {
			// Look for sections that contain rules
			titleLower := strings.ToLower(section.Title)
			if containsRuleIndicator(titleLower) {
				// Extract bullet points as rules
				extractedRules := r.extractBulletPoints(section.Content)
				rules = append(rules, extractedRules...)
			}
		}
	}

	return rules
}

func containsRuleIndicator(title string) bool {
	indicators := []string{
		"rule", "convention", "practice", "guideline",
		"principle", "pattern", "style", "requirement",
		"must", "should", "standard", "code quality",
	}

	for _, indicator := range indicators {
		if strings.Contains(title, indicator) {
			return true
		}
	}
	return false
}

func (r *InstructionsReader) extractBulletPoints(content string) []string {
	rules := make([]string, 0)
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for bullet points
		if strings.HasPrefix(trimmed, "- ") ||
			strings.HasPrefix(trimmed, "* ") ||
			strings.HasPrefix(trimmed, "+ ") {
			rule := strings.TrimPrefix(trimmed, "- ")
			rule = strings.TrimPrefix(rule, "* ")
			rule = strings.TrimPrefix(rule, "+ ")
			rule = strings.TrimSpace(rule)

			if len(rule) > 10 { // Skip very short rules
				rules = append(rules, rule)
			}
		}

		// Check for numbered lists
		if len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9' {
			dotIdx := strings.Index(trimmed, ".")
			if dotIdx > 0 && dotIdx < 3 {
				rule := strings.TrimSpace(trimmed[dotIdx+1:])
				if len(rule) > 10 {
					rules = append(rules, rule)
				}
			}
		}
	}

	return rules
}

// HasScanDirective checks if content contains @scan directive (not @scanned)
func (r *InstructionsReader) HasScanDirective(content string) bool {
	// Check for @scan but not @scanned
	idx := strings.Index(content, "@scan")
	if idx == -1 {
		return false
	}

	// Make sure it's not @scanned by checking the next character
	afterScan := idx + len("@scan")
	if afterScan < len(content) {
		nextChar := content[afterScan]
		// If followed by 'n' (as in @scanned), it's not a valid directive
		if nextChar == 'n' {
			return false
		}
	}

	return true
}

// HasPRMateDirective checks if content contains @prmate directive
func (r *InstructionsReader) HasPRMateDirective(content string) bool {
	return strings.Contains(content, "@prmate")
}

// ParseScanDirective extracts repo addresses from @scan block
func (r *InstructionsReader) ParseScanDirective(content string) []string {
	repos := make([]string, 0)

	// Find the scan block: <!-- PRMate\n@scan\nrepo1\nrepo2\n-->
	startIdx := strings.Index(content, "@scan")
	if startIdx == -1 {
		return repos
	}

	// Find the end marker
	endIdx := strings.Index(content[startIdx:], "-->")
	if endIdx == -1 {
		endIdx = len(content) - startIdx
	}

	block := content[startIdx : startIdx+endIdx]
	lines := strings.Split(block, "\n")

	for _, line := range lines[1:] { // Skip the @scan line itself
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "-->" {
			continue
		}
		// Basic validation: should look like a repo reference
		if strings.Contains(trimmed, "/") || strings.HasPrefix(trimmed, "github.com") {
			repos = append(repos, trimmed)
		}
	}

	return repos
}

// RemoveScanDirective removes @scan from content after processing
func (r *InstructionsReader) RemoveScanDirective(content string) string {
	// Replace @scan with empty to mark as processed
	return strings.Replace(content, "@scan", "@scanned", 1)
}
