package review

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	ghclient "prmate/internal/github"
	"prmate/internal/scanner"
)

const (
	summaryMarkerPrefix = "<!-- prmate-review-summary:"
	summaryMarkerSuffix = " -->"
	summaryVersion      = "1.0"
)

// GitHubClient defines the GitHub operations needed for reviews
type GitHubClient interface {
	GetPullRequest(ctx context.Context, owner, repo string, prNumber int) (*ghclient.PullRequest, error)
	GetPRFiles(ctx context.Context, owner, repo string, prNumber int) ([]ghclient.PRFile, error)
	GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error)
	ListPRComments(ctx context.Context, owner, repo string, prNumber int) ([]string, error)
	ListReviewComments(ctx context.Context, owner, repo string, prNumber int) ([]ghclient.ReviewComment, error)
	CreatePullRequestReview(ctx context.Context, owner, repo string, prNumber int, commitID string, event string, body string, comments []ghclient.DraftReviewComment) error
	CreatePRComment(ctx context.Context, owner, repo string, prNumber int, body string) error
}

// LLMProvider defines the LLM operations needed for analysis
type LLMProvider interface {
	GenerateText(prompt string) (string, error)
}

// InstructionsReader defines the interface for reading instruction files
type InstructionsReader interface {
	ReadPRMateContext(repoPath string) (*scanner.InstructionFile, error)
	ExtractRulesFromInstructions(instructions []scanner.InstructionFile) []string
}

// Service performs PR reviews based on .prmate.md rules
type Service struct {
	githubClient GitHubClient
	llmProvider  LLMProvider
	instReader   *scanner.InstructionsReader
}

// NewService creates a new review service
func NewService(gh GitHubClient, llm LLMProvider) *Service {
	return &Service{
		githubClient: gh,
		llmProvider:  llm,
		instReader:   scanner.NewInstructionsReader(),
	}
}

// ReviewPR performs a complete review of a pull request
func (s *Service) ReviewPR(ctx context.Context, req ReviewRequest) (*ReviewResult, error) {
	log.Printf("Starting review for %s/%s PR #%d (commit: %s)", req.Owner, req.Repo, req.PRNumber, req.HeadSHA[:7])

	// 1. Load rules from .prmate.md
	rules, checklist, codebaseInfo, err := s.loadRules(ctx, req.Owner, req.Repo, req.HeadRef)
	if err != nil {
		return nil, fmt.Errorf("load rules: %w", err)
	}

	if len(rules) == 0 && len(checklist) == 0 {
		log.Printf("No rules found in .prmate.md, skipping review")
		return &ReviewResult{ReviewedCommit: req.HeadSHA}, nil
	}

	log.Printf("Loaded %d rules and %d checklist items", len(rules), len(checklist))

	// 2. Get previous review summary to identify already-reviewed files
	previousSummary, err := s.getPreviousSummary(ctx, req.Owner, req.Repo, req.PRNumber)
	if err != nil {
		log.Printf("Warning: could not get previous summary: %v", err)
	}

	// 3. Get changed files
	files, err := s.githubClient.GetPRFiles(ctx, req.Owner, req.Repo, req.PRNumber)
	if err != nil {
		return nil, fmt.Errorf("get pr files: %w", err)
	}

	// 4. Filter files to review (skip already reviewed unchanged files)
	filesToReview := s.filterFilesToReview(files, previousSummary, req.HeadSHA)
	log.Printf("Reviewing %d of %d changed files", len(filesToReview), len(files))

	// 5. Analyze each file
	var allViolations []FileViolation
	fileStatuses := make([]FileReviewStatus, 0, len(filesToReview))

	for _, file := range filesToReview {
		if file.Status == "removed" {
			continue // Skip deleted files
		}

		violations, err := s.analyzeFile(ctx, req, file, rules, checklist, codebaseInfo)
		if err != nil {
			log.Printf("Warning: failed to analyze %s: %v", file.Filename, err)
			continue
		}

		allViolations = append(allViolations, violations...)
		fileStatuses = append(fileStatuses, FileReviewStatus{
			Path:       file.Filename,
			LastSHA:    req.HeadSHA,
			Violations: len(violations),
			ReviewedAt: time.Now().Format(time.RFC3339),
		})
	}

	// 6. Post review with comments
	var commentsPosted int
	if len(allViolations) > 0 {
		commentsPosted, err = s.postReviewComments(ctx, req, allViolations)
		if err != nil {
			log.Printf("Warning: failed to post review comments: %v", err)
		}
	}

	// 7. Post summary
	summary := ReviewSummary{
		Version:         summaryVersion,
		LastReviewedAt:  time.Now(),
		HeadSHA:         req.HeadSHA,
		FilesScanned:    fileStatuses,
		RulesApplied:    len(rules) + len(checklist),
		ViolationsFound: len(allViolations),
	}

	if err := s.postSummary(ctx, req, summary); err != nil {
		log.Printf("Warning: failed to post summary: %v", err)
	}

	return &ReviewResult{
		FilesReviewed:   len(filesToReview),
		CommentsPosted:  commentsPosted,
		ViolationsFound: len(allViolations),
		SummaryPosted:   true,
		ReviewedCommit:  req.HeadSHA,
	}, nil
}

// loadRules fetches and parses .prmate.md from the repository
func (s *Service) loadRules(ctx context.Context, owner, repo, ref string) (rules []string, checklist []string, codebaseInfo string, err error) {
	content, err := s.githubClient.GetFileContent(ctx, owner, repo, ".prmate.md", ref)
	if err != nil {
		return nil, nil, "", fmt.Errorf("get .prmate.md: %w", err)
	}

	// Parse the content manually since we have the raw content
	sections := parseMarkdownSections(content)

	for _, section := range sections {
		titleLower := strings.ToLower(section.Title)

		// Extract checklist items
		if strings.Contains(titleLower, "checklist") || strings.Contains(titleLower, "review") {
			checklist = append(checklist, extractChecklistItems(section.Content)...)
		}

		// Extract learned rules
		if strings.Contains(titleLower, "rule") || strings.Contains(titleLower, "convention") {
			rules = append(rules, extractBulletPoints(section.Content)...)
		}

		// Collect codebase info sections
		if strings.Contains(titleLower, "structure") ||
			strings.Contains(titleLower, "abstraction") ||
			strings.Contains(titleLower, "naming") ||
			strings.Contains(titleLower, "error") {
			codebaseInfo += fmt.Sprintf("\n## %s\n%s\n", section.Title, section.Content)
		}
	}

	return rules, checklist, codebaseInfo, nil
}

// getPreviousSummary retrieves the last review summary from PR comments
func (s *Service) getPreviousSummary(ctx context.Context, owner, repo string, prNumber int) (*ReviewSummary, error) {
	comments, err := s.githubClient.ListPRComments(ctx, owner, repo, prNumber)
	if err != nil {
		return nil, err
	}

	// Find the latest prmate summary comment
	for i := len(comments) - 1; i >= 0; i-- {
		if strings.Contains(comments[i], summaryMarkerPrefix) {
			return parseSummaryFromComment(comments[i])
		}
	}

	return nil, nil
}

// filterFilesToReview returns files that need review (new or changed since last review)
func (s *Service) filterFilesToReview(files []ghclient.PRFile, previousSummary *ReviewSummary, currentSHA string) []ghclient.PRFile {
	if previousSummary == nil {
		return files // Review all files if no previous summary
	}

	// Build a map of previously reviewed files
	reviewed := make(map[string]string) // path -> last reviewed SHA
	for _, f := range previousSummary.FilesScanned {
		reviewed[f.Path] = f.LastSHA
	}

	// Filter to files that need review
	var toReview []ghclient.PRFile
	for _, file := range files {
		lastSHA, wasReviewed := reviewed[file.Filename]
		// Review if never reviewed or if the file has changes (patch exists means changes)
		if !wasReviewed || lastSHA != currentSHA || file.Patch != "" {
			toReview = append(toReview, file)
		}
	}

	return toReview
}

// analyzeFile uses LLM to analyze a single file against rules
func (s *Service) analyzeFile(ctx context.Context, req ReviewRequest, file ghclient.PRFile, rules, checklist []string, codebaseInfo string) ([]FileViolation, error) {
	// Get full file content for context (if not too large)
	var fileContent string
	if file.Additions+file.Deletions < 500 {
		content, err := s.githubClient.GetFileContent(ctx, req.Owner, req.Repo, file.Filename, req.HeadRef)
		if err == nil {
			fileContent = content
		}
	}

	// Get dependency context - files that this file imports/references
	dependencyContext := s.gatherDependencyContext(ctx, req, file.Filename, fileContent)

	// Build the analysis prompt with dependency context
	prompt := s.buildAnalysisPrompt(file.Filename, fileContent, file.Patch, rules, checklist, codebaseInfo, dependencyContext)

	// Call LLM
	response, err := s.llmProvider.GenerateText(prompt)
	if err != nil {
		return nil, fmt.Errorf("llm analysis: %w", err)
	}

	// Parse LLM response
	violations := s.parseLLMResponse(response, file.Filename, file.Patch)

	return violations, nil
}

// gatherDependencyContext fetches content from files that the changed file depends on
func (s *Service) gatherDependencyContext(ctx context.Context, req ReviewRequest, filePath, fileContent string) string {
	if fileContent == "" {
		return ""
	}

	var dependencies []string

	// Detect language and extract imports
	ext := getFileExtension(filePath)
	switch ext {
	case ".go":
		dependencies = extractGoImports(fileContent, filePath)
	case ".ts", ".tsx", ".js", ".jsx":
		dependencies = extractJSImports(fileContent, filePath)
	case ".py":
		dependencies = extractPythonImports(fileContent, filePath)
	}

	if len(dependencies) == 0 {
		return ""
	}

	var sb strings.Builder
	fetchedCount := 0
	maxDeps := 5 // Limit to avoid token explosion

	for _, depPath := range dependencies {
		if fetchedCount >= maxDeps {
			break
		}

		content, err := s.githubClient.GetFileContent(ctx, req.Owner, req.Repo, depPath, req.HeadRef)
		if err != nil {
			continue // File might not exist or be external
		}

		// Truncate large files to keep prompt reasonable
		if len(content) > 3000 {
			content = content[:3000] + "\n// ... (truncated)"
		}

		sb.WriteString(fmt.Sprintf("\n### %s\n```\n%s\n```\n", depPath, content))
		fetchedCount++
	}

	return sb.String()
}

// getFileExtension returns the file extension including the dot
func getFileExtension(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
		if path[i] == '/' {
			break
		}
	}
	return ""
}

// extractGoImports finds local package imports from Go source
func extractGoImports(content, currentFile string) []string {
	var deps []string

	// Find import block
	inImport := false
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Single import
		if strings.HasPrefix(trimmed, "import \"") || strings.HasPrefix(trimmed, "import `") {
			continue // Skip, likely stdlib
		}

		// Start of import block
		if trimmed == "import (" {
			inImport = true
			continue
		}

		// End of import block
		if inImport && trimmed == ")" {
			inImport = false
			continue
		}

		// Inside import block
		if inImport {
			// Look for local imports (containing the module path)
			trimmed = strings.Trim(trimmed, "\t \"'`")
			// Skip empty lines and comments
			if trimmed == "" || strings.HasPrefix(trimmed, "//") {
				continue
			}
			// Check if it's a local import (has internal/ or similar pattern)
			if strings.Contains(trimmed, "/internal/") || strings.Contains(trimmed, "/pkg/") {
				// Convert import path to file path
				parts := strings.Split(trimmed, "/")
				for i, part := range parts {
					if part == "internal" || part == "pkg" {
						localPath := strings.Join(parts[i:], "/")
						// Try common file patterns
						deps = append(deps, localPath+".go")
						// Also check for types.go which often has interfaces
						if strings.HasSuffix(localPath, "/") {
							deps = append(deps, localPath+"types.go")
						} else {
							// Package folder - look for common files
							deps = append(deps, localPath+"/types.go")
							deps = append(deps, localPath+"/service.go")
						}
						break
					}
				}
			}
		}
	}

	// Also look for the types.go in same package
	dir := getDirectory(currentFile)
	if dir != "" {
		deps = append(deps, dir+"/types.go")
	}

	return deps
}

// extractJSImports finds local imports from JavaScript/TypeScript source
func extractJSImports(content, currentFile string) []string {
	var deps []string
	dir := getDirectory(currentFile)

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Match: import ... from './something' or import ... from '../something'
		if strings.Contains(trimmed, "from '") || strings.Contains(trimmed, "from \"") {
			start := strings.Index(trimmed, "from ")
			if start == -1 {
				continue
			}
			rest := trimmed[start+5:]
			rest = strings.Trim(rest, "'\";")

			// Only local imports (starting with . or ..)
			if strings.HasPrefix(rest, "./") || strings.HasPrefix(rest, "../") {
				// Resolve relative path
				resolved := resolveRelativePath(dir, rest)
				if resolved != "" {
					// Try with common extensions
					deps = append(deps, resolved+".ts", resolved+".tsx", resolved+".js", resolved+"/index.ts")
				}
			}
		}
	}

	return deps
}

// extractPythonImports finds local imports from Python source
func extractPythonImports(content, currentFile string) []string {
	var deps []string
	dir := getDirectory(currentFile)

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Match: from .module import something or from ..module import something
		if strings.HasPrefix(trimmed, "from .") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				module := parts[1]
				if strings.HasPrefix(module, ".") {
					// Relative import
					module = strings.TrimLeft(module, ".")
					module = strings.Replace(module, ".", "/", -1)
					resolved := dir + "/" + module + ".py"
					deps = append(deps, resolved)
				}
			}
		}
	}

	return deps
}

// getDirectory returns the directory part of a file path
func getDirectory(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return ""
}

// resolveRelativePath resolves a relative import path
func resolveRelativePath(baseDir, relativePath string) string {
	if strings.HasPrefix(relativePath, "./") {
		return baseDir + "/" + relativePath[2:]
	}
	if strings.HasPrefix(relativePath, "../") {
		// Go up one directory
		parentDir := getDirectory(baseDir)
		if parentDir == "" {
			return ""
		}
		return resolveRelativePath(parentDir, relativePath[3:])
	}
	return baseDir + "/" + relativePath
}

// buildAnalysisPrompt constructs the prompt for LLM analysis
func (s *Service) buildAnalysisPrompt(filePath, fileContent, patch string, rules, checklist []string, codebaseInfo string, dependencyContext string) string {
	var sb strings.Builder

	sb.WriteString("You are a senior code reviewer. Analyze the following code changes and identify any violations of the project's coding standards.\n\n")

	sb.WriteString("## Project Rules and Conventions\n")
	for i, rule := range rules {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, rule))
	}

	if len(checklist) > 0 {
		sb.WriteString("\n## Review Checklist\n")
		for _, item := range checklist {
			sb.WriteString(fmt.Sprintf("- [ ] %s\n", item))
		}
	}

	if codebaseInfo != "" {
		sb.WriteString("\n## Codebase Context\n")
		sb.WriteString(codebaseInfo)
	}

	if dependencyContext != "" {
		sb.WriteString("\n## Related Files (Dependencies/Interfaces)\n")
		sb.WriteString("Use this context to understand types, interfaces, and patterns the changed code should follow:\n")
		sb.WriteString(dependencyContext)
	}

	sb.WriteString(fmt.Sprintf("\n## File Being Reviewed: %s\n", filePath))

	if patch != "" {
		sb.WriteString("\n### Changes (Diff)\n```diff\n")
		sb.WriteString(patch)
		sb.WriteString("\n```\n")
	}

	if fileContent != "" && len(fileContent) < 10000 {
		sb.WriteString("\n### Full File Content\n```\n")
		sb.WriteString(fileContent)
		sb.WriteString("\n```\n")
	}

	sb.WriteString(`
## Response Format
Respond with a JSON object containing violations found. Only report violations for ADDED or MODIFIED lines (lines starting with + in the diff).
If no violations are found, return {"violations": []}.

Example response:
{"violations": [{"line": 42, "rule": "Error Handling", "message": "Error not wrapped with context", "severity": "warning", "fix": "Use fmt.Errorf(\"context: %w\", err)"}]}

Important:
- Only flag clear violations, not style preferences
- Line numbers should reference the NEW file line numbers (from lines starting with +)
- Be specific about what rule is violated and how to fix it
- Severity: "error" for breaking issues, "warning" for best practices, "suggestion" for improvements
- Check that the code correctly implements interfaces and follows patterns from the dependency context

Respond with ONLY the JSON, no additional text.
`)

	return sb.String()
}

// parseLLMResponse extracts violations from LLM response
func (s *Service) parseLLMResponse(response, filePath, patch string) []FileViolation {
	// Clean up the response - remove markdown code blocks if present
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var llmResp LLMAnalysisResponse
	if err := json.Unmarshal([]byte(response), &llmResp); err != nil {
		log.Printf("Warning: failed to parse LLM response: %v", err)
		return nil
	}

	// Get valid line numbers from the patch
	validLines := make(map[int]bool)
	for _, lineNo := range ghclient.GetNewLineNumbers(patch) {
		validLines[lineNo] = true
	}

	violations := make([]FileViolation, 0, len(llmResp.Violations))
	for _, v := range llmResp.Violations {
		// Validate that the line number is in the patch
		if !validLines[v.Line] && len(validLines) > 0 {
			continue // Skip violations on lines not in the diff
		}

		violations = append(violations, FileViolation{
			Path:     filePath,
			Line:     v.Line,
			Rule:     v.Rule,
			Message:  v.Message,
			Severity: v.Severity,
		})
	}

	return violations
}

// postReviewComments creates a GitHub review with inline comments
func (s *Service) postReviewComments(ctx context.Context, req ReviewRequest, violations []FileViolation) (int, error) {
	if len(violations) == 0 {
		return 0, nil
	}

	comments := make([]ghclient.DraftReviewComment, 0, len(violations))

	for _, v := range violations {
		emoji := "⚠️"
		if v.Severity == "error" {
			emoji = "❌"
		} else if v.Severity == "suggestion" {
			emoji = "💡"
		}

		body := fmt.Sprintf("%s **%s**: %s", emoji, v.Rule, v.Message)

		comments = append(comments, ghclient.DraftReviewComment{
			Path: v.Path,
			Line: v.Line,
			Side: "RIGHT",
			Body: body,
		})
	}

	reviewBody := fmt.Sprintf("🔍 **PRMate Review** - Found %d issue(s) to address.", len(violations))

	// Determine review event based on severity
	event := "COMMENT"
	for _, v := range violations {
		if v.Severity == "error" {
			event = "REQUEST_CHANGES"
			break
		}
	}

	err := s.githubClient.CreatePullRequestReview(ctx, req.Owner, req.Repo, req.PRNumber, req.HeadSHA, event, reviewBody, comments)
	if err != nil {
		return 0, err
	}

	return len(comments), nil
}

// postSummary creates a PR comment with the review summary
func (s *Service) postSummary(ctx context.Context, req ReviewRequest, summary ReviewSummary) error {
	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("marshal summary: %w", err)
	}

	var sb strings.Builder

	// Hidden marker for parsing
	sb.WriteString(fmt.Sprintf("%s%s%s\n", summaryMarkerPrefix, req.HeadSHA, summaryMarkerSuffix))

	// Human-readable summary
	sb.WriteString("## 📊 PRMate Review Summary\n\n")
	sb.WriteString(fmt.Sprintf("| Metric | Value |\n|--------|-------|\n"))
	sb.WriteString(fmt.Sprintf("| Files Reviewed | %d |\n", len(summary.FilesScanned)))
	sb.WriteString(fmt.Sprintf("| Rules Applied | %d |\n", summary.RulesApplied))
	sb.WriteString(fmt.Sprintf("| Issues Found | %d |\n", summary.ViolationsFound))
	sb.WriteString(fmt.Sprintf("| Commit | `%s` |\n", summary.HeadSHA[:7]))

	if len(summary.FilesScanned) > 0 {
		sb.WriteString("\n<details>\n<summary>Files Reviewed</summary>\n\n")
		for _, f := range summary.FilesScanned {
			status := "✅"
			if f.Violations > 0 {
				status = fmt.Sprintf("⚠️ %d issue(s)", f.Violations)
			}
			sb.WriteString(fmt.Sprintf("- `%s` %s\n", f.Path, status))
		}
		sb.WriteString("</details>\n")
	}

	// Hidden JSON data for future parsing
	sb.WriteString(fmt.Sprintf("\n<!-- prmate-data:%s -->", string(summaryJSON)))

	return s.githubClient.CreatePRComment(ctx, req.Owner, req.Repo, req.PRNumber, sb.String())
}

// parseSummaryFromComment extracts ReviewSummary from a comment body
func parseSummaryFromComment(comment string) (*ReviewSummary, error) {
	// Find the JSON data marker
	marker := "<!-- prmate-data:"
	idx := strings.Index(comment, marker)
	if idx == -1 {
		return nil, fmt.Errorf("no summary data found")
	}

	start := idx + len(marker)
	end := strings.Index(comment[start:], " -->")
	if end == -1 {
		return nil, fmt.Errorf("malformed summary data")
	}

	jsonData := comment[start : start+end]

	var summary ReviewSummary
	if err := json.Unmarshal([]byte(jsonData), &summary); err != nil {
		return nil, fmt.Errorf("parse summary json: %w", err)
	}

	return &summary, nil
}

// Helper functions for parsing markdown

type markdownSection struct {
	Title   string
	Content string
	Level   int
}

func parseMarkdownSections(content string) []markdownSection {
	sections := make([]markdownSection, 0)
	lines := strings.Split(content, "\n")

	var currentSection *markdownSection
	var contentBuilder strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "#") {
			if currentSection != nil {
				currentSection.Content = strings.TrimSpace(contentBuilder.String())
				sections = append(sections, *currentSection)
				contentBuilder.Reset()
			}

			level := 0
			for _, c := range trimmed {
				if c == '#' {
					level++
				} else {
					break
				}
			}

			title := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			currentSection = &markdownSection{Title: title, Level: level}
		} else if currentSection != nil {
			contentBuilder.WriteString(line)
			contentBuilder.WriteString("\n")
		}
	}

	if currentSection != nil {
		currentSection.Content = strings.TrimSpace(contentBuilder.String())
		sections = append(sections, *currentSection)
	}

	return sections
}

func extractBulletPoints(content string) []string {
	rules := make([]string, 0)
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "- ") ||
			strings.HasPrefix(trimmed, "* ") ||
			strings.HasPrefix(trimmed, "+ ") {
			rule := strings.TrimPrefix(trimmed, "- ")
			rule = strings.TrimPrefix(rule, "* ")
			rule = strings.TrimPrefix(rule, "+ ")
			rule = strings.TrimSpace(rule)

			if len(rule) > 10 {
				rules = append(rules, rule)
			}
		}
	}

	return rules
}

func extractChecklistItems(content string) []string {
	items := make([]string, 0)
	
	// Match checkbox items: - [ ] item or - [x] item
	re := regexp.MustCompile(`-\s*\[[ x]\]\s*(.+)`)
	matches := re.FindAllStringSubmatch(content, -1)
	
	for _, match := range matches {
		if len(match) > 1 && len(match[1]) > 5 {
			items = append(items, strings.TrimSpace(match[1]))
		}
	}

	return items
}

// HasPRMateFile checks if a .prmate.md file exists in the repository
func (s *Service) HasPRMateFile(ctx context.Context, owner, repo, ref string) bool {
	_, err := s.githubClient.GetFileContent(ctx, owner, repo, ".prmate.md", ref)
	return err == nil
}
