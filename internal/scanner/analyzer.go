package scanner

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// NamingStyle represents detected naming convention
type NamingStyle string

const (
	NamingCamelCase  NamingStyle = "camelCase"
	NamingPascalCase NamingStyle = "PascalCase"
	NamingSnakeCase  NamingStyle = "snake_case"
	NamingKebabCase  NamingStyle = "kebab-case"
	NamingMixed      NamingStyle = "mixed"
)

// PatternMatch represents a detected pattern with examples
type PatternMatch struct {
	Pattern  string
	Examples []string
	Count    int
}

// AbstractionInfo describes an abstraction layer in the codebase
type AbstractionInfo struct {
	Name        string   // e.g., "Service", "Handler", "Repository"
	Suffix      string   // e.g., "Service", "Handler"
	Prefix      string   // e.g., "I" for interfaces
	Locations   []string // file paths where found
	IsInterface bool
}

// FolderConvention describes folder structure patterns
type FolderConvention struct {
	Pattern     string   // e.g., "internal/{domain}/"
	Purpose     string   // e.g., "Domain services"
	Examples    []string
	Depth       int
}

// ErrorPattern describes error handling patterns
type ErrorPattern struct {
	Style       string   // "wrap", "raw", "custom"
	Examples    []string
	Count       int
}

// AnalysisResult contains all detected patterns
type AnalysisResult struct {
	FolderNaming      NamingStyle
	FileNaming        NamingStyle
	FolderConventions []FolderConvention
	Abstractions      []AbstractionInfo
	NamingPatterns    []PatternMatch
	ErrorPatterns     []ErrorPattern
	TestConventions   TestConvention
	ImportPatterns    []string
}

// TestConvention describes how tests are organized
type TestConvention struct {
	Colocated      bool   // tests in same folder as source
	SeparateFolder bool   // tests in separate test/ folder
	TestSuffix     string // e.g., "_test.go"
	Examples       []string
}

// Analyzer extracts patterns from a CodebaseContext
type Analyzer struct{}

// NewAnalyzer creates a new pattern analyzer
func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

// Analyze extracts patterns from a scanned codebase
func (a *Analyzer) Analyze(ctx *CodebaseContext) (*AnalysisResult, error) {
	result := &AnalysisResult{
		FolderConventions: make([]FolderConvention, 0),
		Abstractions:      make([]AbstractionInfo, 0),
		NamingPatterns:    make([]PatternMatch, 0),
		ErrorPatterns:     make([]ErrorPattern, 0),
		ImportPatterns:    make([]string, 0),
	}

	// Analyze folder naming
	result.FolderNaming = a.analyzeFolderNaming(ctx)

	// Analyze file naming
	result.FileNaming = a.analyzeFileNaming(ctx)

	// Detect folder conventions
	result.FolderConventions = a.detectFolderConventions(ctx)

	// Detect abstractions (services, handlers, etc.)
	result.Abstractions = a.detectAbstractions(ctx)

	// Detect naming patterns
	result.NamingPatterns = a.detectNamingPatterns(ctx)

	// Detect error handling patterns
	result.ErrorPatterns = a.detectErrorPatterns(ctx)

	// Detect test conventions
	result.TestConventions = a.detectTestConventions(ctx)

	return result, nil
}

func (a *Analyzer) analyzeFolderNaming(ctx *CodebaseContext) NamingStyle {
	styles := make(map[NamingStyle]int)

	for _, folder := range ctx.TopLevelFolders {
		style := detectNamingStyle(folder)
		styles[style]++
	}

	return dominantStyle(styles)
}

func (a *Analyzer) analyzeFileNaming(ctx *CodebaseContext) NamingStyle {
	styles := make(map[NamingStyle]int)

	for _, file := range ctx.Files {
		name := strings.TrimSuffix(file.Name, file.Extension)
		style := detectNamingStyle(name)
		styles[style]++
	}

	return dominantStyle(styles)
}

func detectNamingStyle(name string) NamingStyle {
	if strings.Contains(name, "-") {
		return NamingKebabCase
	}
	if strings.Contains(name, "_") {
		return NamingSnakeCase
	}
	if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
		return NamingPascalCase
	}
	if len(name) > 0 && name[0] >= 'a' && name[0] <= 'z' {
		return NamingCamelCase
	}
	return NamingMixed
}

func dominantStyle(styles map[NamingStyle]int) NamingStyle {
	var maxStyle NamingStyle
	var maxCount int

	for style, count := range styles {
		if count > maxCount {
			maxStyle = style
			maxCount = count
		}
	}

	if maxStyle == "" {
		return NamingMixed
	}
	return maxStyle
}

func (a *Analyzer) detectFolderConventions(ctx *CodebaseContext) []FolderConvention {
	conventions := make([]FolderConvention, 0)

	// Check for common Go patterns
	for _, folder := range ctx.TopLevelFolders {
		switch folder {
		case "internal":
			conv := FolderConvention{
				Pattern:  "internal/{domain}/",
				Purpose:  "Private application code organized by domain",
				Depth:    1,
				Examples: a.findSubfolders(ctx, "internal"),
			}
			conventions = append(conventions, conv)

		case "pkg":
			conv := FolderConvention{
				Pattern:  "pkg/{library}/",
				Purpose:  "Public reusable packages",
				Depth:    1,
				Examples: a.findSubfolders(ctx, "pkg"),
			}
			conventions = append(conventions, conv)

		case "cmd":
			conv := FolderConvention{
				Pattern:  "cmd/{app}/",
				Purpose:  "Application entry points",
				Depth:    1,
				Examples: a.findSubfolders(ctx, "cmd"),
			}
			conventions = append(conventions, conv)

		case "api":
			conv := FolderConvention{
				Pattern:  "api/",
				Purpose:  "API definitions (OpenAPI, protobuf)",
				Depth:    1,
				Examples: []string{folder},
			}
			conventions = append(conventions, conv)

		case "configs", "config":
			conv := FolderConvention{
				Pattern:  folder + "/",
				Purpose:  "Configuration files",
				Depth:    1,
				Examples: []string{folder},
			}
			conventions = append(conventions, conv)
		}
	}

	return conventions
}

func (a *Analyzer) findSubfolders(ctx *CodebaseContext, parent string) []string {
	subfolders := make([]string, 0)
	prefix := filepath.Join(ctx.RootPath, parent)

	for depth, folders := range ctx.FoldersByDepth {
		if depth == 2 { // Looking for immediate children of parent
			for _, folder := range folders {
				if strings.HasPrefix(folder, prefix) {
					subfolders = append(subfolders, filepath.Base(folder))
				}
			}
		}
	}

	return subfolders
}

func (a *Analyzer) detectAbstractions(ctx *CodebaseContext) []AbstractionInfo {
	abstractions := make(map[string]*AbstractionInfo)

	suffixes := []string{"Service", "Handler", "Repository", "Client", "Manager", "Controller", "Provider"}

	for _, file := range ctx.Files {
		if file.Extension != ".go" {
			continue
		}

		name := strings.TrimSuffix(file.Name, file.Extension)

		for _, suffix := range suffixes {
			if strings.HasSuffix(name, suffix) || strings.HasSuffix(strings.ToLower(name), strings.ToLower(suffix)) {
				key := strings.ToLower(suffix)
				if abstractions[key] == nil {
					abstractions[key] = &AbstractionInfo{
						Name:      suffix,
						Suffix:    suffix,
						Locations: make([]string, 0),
					}
				}
				abstractions[key].Locations = append(abstractions[key].Locations, file.Path)
			}
		}

		// Check for interfaces
		content, err := os.ReadFile(file.Path)
		if err == nil && strings.Contains(string(content), "type ") && strings.Contains(string(content), " interface {") {
			if abstractions["interface"] == nil {
				abstractions["interface"] = &AbstractionInfo{
					Name:        "Interface",
					IsInterface: true,
					Locations:   make([]string, 0),
				}
			}
			abstractions["interface"].Locations = append(abstractions["interface"].Locations, file.Path)
		}
	}

	result := make([]AbstractionInfo, 0, len(abstractions))
	for _, abs := range abstractions {
		result = append(result, *abs)
	}

	return result
}

func (a *Analyzer) detectNamingPatterns(ctx *CodebaseContext) []PatternMatch {
	patterns := make([]PatternMatch, 0)

	// Detect common suffixes
	suffixCounts := make(map[string][]string)
	for _, file := range ctx.Files {
		if file.Extension != ".go" {
			continue
		}
		name := strings.TrimSuffix(file.Name, file.Extension)

		// Check for common suffixes
		for _, suffix := range []string{"_test", "Service", "Handler", "Repository", "Client", "Manager"} {
			if strings.HasSuffix(name, suffix) {
				suffixCounts[suffix] = append(suffixCounts[suffix], name)
			}
		}
	}

	for suffix, examples := range suffixCounts {
		if len(examples) > 0 {
			patterns = append(patterns, PatternMatch{
				Pattern:  "*" + suffix,
				Examples: examples,
				Count:    len(examples),
			})
		}
	}

	return patterns
}

func (a *Analyzer) detectErrorPatterns(ctx *CodebaseContext) []ErrorPattern {
	patterns := make(map[string]*ErrorPattern)

	wrapRegex := regexp.MustCompile(`fmt\.Errorf\([^)]*%w`)
	rawRegex := regexp.MustCompile(`return\s+err\s*$`)

	for _, file := range ctx.Files {
		if file.Extension != ".go" {
			continue
		}

		content, err := os.ReadFile(file.Path)
		if err != nil {
			continue
		}

		contentStr := string(content)

		// Check for wrapped errors
		if wrapRegex.MatchString(contentStr) {
			if patterns["wrap"] == nil {
				patterns["wrap"] = &ErrorPattern{Style: "wrap", Examples: make([]string, 0)}
			}
			patterns["wrap"].Count++
			if len(patterns["wrap"].Examples) < 3 {
				patterns["wrap"].Examples = append(patterns["wrap"].Examples, file.Path)
			}
		}

		// Check for raw error returns
		if rawRegex.MatchString(contentStr) {
			if patterns["raw"] == nil {
				patterns["raw"] = &ErrorPattern{Style: "raw", Examples: make([]string, 0)}
			}
			patterns["raw"].Count++
			if len(patterns["raw"].Examples) < 3 {
				patterns["raw"].Examples = append(patterns["raw"].Examples, file.Path)
			}
		}
	}

	result := make([]ErrorPattern, 0, len(patterns))
	for _, pattern := range patterns {
		result = append(result, *pattern)
	}

	return result
}

func (a *Analyzer) detectTestConventions(ctx *CodebaseContext) TestConvention {
	conv := TestConvention{
		TestSuffix: "_test.go",
		Examples:   make([]string, 0),
	}

	testFiles := make([]string, 0)
	sourceFiles := make(map[string]bool)

	for _, file := range ctx.Files {
		if file.Extension != ".go" {
			continue
		}

		if strings.HasSuffix(file.Name, "_test.go") {
			testFiles = append(testFiles, file.Path)
		} else {
			dir := filepath.Dir(file.Path)
			sourceFiles[dir] = true
		}
	}

	// Check if tests are colocated
	colocatedCount := 0
	separateCount := 0

	for _, testPath := range testFiles {
		dir := filepath.Dir(testPath)
		if sourceFiles[dir] {
			colocatedCount++
		} else {
			separateCount++
		}

		if len(conv.Examples) < 3 {
			conv.Examples = append(conv.Examples, testPath)
		}
	}

	conv.Colocated = colocatedCount > separateCount
	conv.SeparateFolder = separateCount > colocatedCount

	return conv
}
