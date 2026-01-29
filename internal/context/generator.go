package context

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"prmate/internal/scanner"
)

// Generator creates .prmate.md content from scan results
type Generator struct{}

// NewGenerator creates a new context generator
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate creates the .prmate.md content from multi-repo scan results
func (g *Generator) Generate(result *scanner.MultiRepoResult) string {
	var sb strings.Builder

	sb.WriteString("# PRMate Context\n\n")
	sb.WriteString("*Auto-generated PR review context. Do not edit directly.*\n\n")

	// Folder Structure section
	g.writeFolderStructure(&sb, result.CurrentRepo, result.CurrentAnalysis)

	// Naming Conventions section
	g.writeNamingConventions(&sb, result.CurrentAnalysis)

	// Abstractions section
	g.writeAbstractions(&sb, result.CurrentAnalysis)

	// Error Handling section
	g.writeErrorHandling(&sb, result.CurrentAnalysis)

	// Test Conventions section
	g.writeTestConventions(&sb, result.CurrentAnalysis)

	// Senior Developer Checklist section
	g.writeSeniorDevChecklist(&sb)

	// Learned Rules section (from instruction files)
	if len(result.MergedRules) > 0 {
		g.writeLearnedRules(&sb, result.MergedRules)
	}

	// Source repos section
	g.writeSourceRepos(&sb, result)

	return sb.String()
}

func (g *Generator) writeFolderStructure(sb *strings.Builder, ctx *scanner.CodebaseContext, analysis *scanner.AnalysisResult) {
	sb.WriteString("## Folder Structure\n\n")

	if len(analysis.FolderConventions) > 0 {
		for _, conv := range analysis.FolderConventions {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", conv.Pattern, conv.Purpose))
			if len(conv.Examples) > 0 {
				examples := conv.Examples
				if len(examples) > 5 {
					examples = examples[:5]
				}
				sb.WriteString(fmt.Sprintf("  - Examples: `%s`\n", strings.Join(examples, "`, `")))
			}
		}
	}

	// Top-level folders
	if len(ctx.TopLevelFolders) > 0 {
		sb.WriteString("\n**Top-level directories:**\n")
		for _, folder := range ctx.TopLevelFolders {
			sb.WriteString(fmt.Sprintf("- `%s/`\n", folder))
		}
	}

	// File extensions breakdown
	if len(ctx.Extensions) > 0 {
		sb.WriteString("\n**File types:**\n")

		// Sort by count
		type extCount struct {
			ext   string
			count int
		}
		counts := make([]extCount, 0, len(ctx.Extensions))
		for ext, count := range ctx.Extensions {
			counts = append(counts, extCount{ext, count})
		}
		sort.Slice(counts, func(i, j int) bool {
			return counts[i].count > counts[j].count
		})

		for i, ec := range counts {
			if i >= 10 {
				break
			}
			sb.WriteString(fmt.Sprintf("- `%s`: %d files\n", ec.ext, ec.count))
		}
	}

	sb.WriteString("\n")
}

func (g *Generator) writeNamingConventions(sb *strings.Builder, analysis *scanner.AnalysisResult) {
	sb.WriteString("## Naming Conventions\n\n")

	sb.WriteString(fmt.Sprintf("- **Folder naming**: %s\n", analysis.FolderNaming))
	sb.WriteString(fmt.Sprintf("- **File naming**: %s\n", analysis.FileNaming))

	if len(analysis.NamingPatterns) > 0 {
		sb.WriteString("\n**Detected patterns:**\n")
		for _, pattern := range analysis.NamingPatterns {
			if pattern.Count > 1 {
				examples := pattern.Examples
				if len(examples) > 3 {
					examples = examples[:3]
				}
				sb.WriteString(fmt.Sprintf("- `%s` (%d occurrences): %s\n",
					pattern.Pattern, pattern.Count, strings.Join(examples, ", ")))
			}
		}
	}

	sb.WriteString("\n")
}

func (g *Generator) writeAbstractions(sb *strings.Builder, analysis *scanner.AnalysisResult) {
	sb.WriteString("## Abstractions\n\n")

	if len(analysis.Abstractions) == 0 {
		sb.WriteString("*No specific abstraction patterns detected.*\n\n")
		return
	}

	// Group by type
	services := make([]scanner.AbstractionInfo, 0)
	handlers := make([]scanner.AbstractionInfo, 0)
	interfaces := make([]scanner.AbstractionInfo, 0)
	others := make([]scanner.AbstractionInfo, 0)

	for _, abs := range analysis.Abstractions {
		switch {
		case abs.IsInterface:
			interfaces = append(interfaces, abs)
		case abs.Name == "Service":
			services = append(services, abs)
		case abs.Name == "Handler":
			handlers = append(handlers, abs)
		default:
			others = append(others, abs)
		}
	}

	if len(services) > 0 {
		sb.WriteString("**Services:**\n")
		for _, svc := range services {
			sb.WriteString(fmt.Sprintf("- `*%s` suffix (%d files)\n", svc.Suffix, len(svc.Locations)))
		}
		sb.WriteString("\n")
	}

	if len(handlers) > 0 {
		sb.WriteString("**Handlers:**\n")
		for _, h := range handlers {
			sb.WriteString(fmt.Sprintf("- `*%s` suffix (%d files)\n", h.Suffix, len(h.Locations)))
		}
		sb.WriteString("\n")
	}

	if len(interfaces) > 0 {
		sb.WriteString("**Interfaces:**\n")
		sb.WriteString(fmt.Sprintf("- Found in %d files\n", len(interfaces[0].Locations)))
		sb.WriteString("- Define interfaces in consumer packages\n")
		sb.WriteString("\n")
	}

	if len(others) > 0 {
		sb.WriteString("**Other patterns:**\n")
		for _, o := range others {
			sb.WriteString(fmt.Sprintf("- `*%s` (%d files)\n", o.Suffix, len(o.Locations)))
		}
		sb.WriteString("\n")
	}
}

func (g *Generator) writeErrorHandling(sb *strings.Builder, analysis *scanner.AnalysisResult) {
	sb.WriteString("## Error Handling\n\n")

	if len(analysis.ErrorPatterns) == 0 {
		sb.WriteString("*No specific error patterns detected.*\n\n")
		return
	}

	for _, pattern := range analysis.ErrorPatterns {
		switch pattern.Style {
		case "wrap":
			sb.WriteString(fmt.Sprintf("- **Error wrapping**: Wrap errors with context using `fmt.Errorf(\"context: %%w\", err)` (%d occurrences)\n", pattern.Count))
		case "raw":
			sb.WriteString(fmt.Sprintf("- **Raw returns**: Found %d raw error returns (consider wrapping with context)\n", pattern.Count))
		}
	}

	sb.WriteString("\n")
}

func (g *Generator) writeTestConventions(sb *strings.Builder, analysis *scanner.AnalysisResult) {
	sb.WriteString("## Test Conventions\n\n")

	conv := analysis.TestConventions

	sb.WriteString(fmt.Sprintf("- **Test suffix**: `%s`\n", conv.TestSuffix))

	if conv.Colocated {
		sb.WriteString("- **Location**: Tests colocated with source files\n")
	} else if conv.SeparateFolder {
		sb.WriteString("- **Location**: Tests in separate folder\n")
	}

	if len(conv.Examples) > 0 {
		examples := conv.Examples
		if len(examples) > 3 {
			examples = examples[:3]
		}
		sb.WriteString(fmt.Sprintf("- **Examples**: `%s`\n", strings.Join(examples, "`, `")))
	}

	sb.WriteString("\n")
}

func (g *Generator) writeSeniorDevChecklist(sb *strings.Builder) {
	sb.WriteString("## Senior Developer Review Checklist\n\n")

	checklist := []string{
		"**File locations**: New files placed in correct folders per conventions above",
		"**Abstraction usage**: Uses existing services/handlers, doesn't bypass abstractions",
		"**Naming consistency**: Follows established naming patterns (suffixes, casing)",
		"**Interface compliance**: Implements required interfaces, defines new ones in consumer",
		"**Error handling**: Errors wrapped with context, no naked returns",
		"**Test coverage**: Tests colocated/placed correctly, follows naming convention",
		"**Security patterns**: No hardcoded secrets, proper input validation",
		"**Documentation**: Exported functions have comments, complex logic explained",
		"**Dependency injection**: Services injected, not created inline",
		"**Resource cleanup**: Proper use of defer for cleanup, context propagation",
	}

	for _, item := range checklist {
		sb.WriteString(fmt.Sprintf("- [ ] %s\n", item))
	}

	sb.WriteString("\n")
}

func (g *Generator) writeLearnedRules(sb *strings.Builder, rules []string) {
	sb.WriteString("## Learned Rules\n\n")

	// Deduplicate rules
	seen := make(map[string]bool)
	unique := make([]string, 0)
	for _, rule := range rules {
		normalized := strings.ToLower(strings.TrimSpace(rule))
		if !seen[normalized] && len(rule) > 0 {
			seen[normalized] = true
			unique = append(unique, rule)
		}
	}

	for _, rule := range unique {
		sb.WriteString(fmt.Sprintf("- %s\n", rule))
	}

	sb.WriteString("\n")
}

func (g *Generator) writeSourceRepos(sb *strings.Builder, result *scanner.MultiRepoResult) {
	sb.WriteString("## Sources\n\n")

	sb.WriteString(fmt.Sprintf("- **Current repository**: `%s`\n", result.CurrentRepo.RepoName))

	if len(result.ExternalRepos) > 0 {
		sb.WriteString("\n**External repositories scanned:**\n")
		for _, ext := range result.ExternalRepos {
			status := "scanned"
			if ext.Source.HasPRMate {
				status = ".prmate.md found"
			}
			if ext.Error != nil {
				status = fmt.Sprintf("error: %v", ext.Error)
			}
			sb.WriteString(fmt.Sprintf("- `%s` (%s)\n", ext.Source.Address, status))
		}
	}

	sb.WriteString("\n")
}

// WriteToFile writes the generated content to a file
func (g *Generator) WriteToFile(content, repoPath string) error {
	outputPath := filepath.Join(repoPath, ".prmate.md")
	return os.WriteFile(outputPath, []byte(content), 0644)
}

// WriteToTemp writes the generated content to a temp file and returns its path
func (g *Generator) WriteToTemp(content string) (string, error) {
	tmpFile, err := os.CreateTemp("", "prmate-*.md")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(content); err != nil {
		return "", fmt.Errorf("write temp file: %w", err)
	}

	return tmpFile.Name(), nil
}
