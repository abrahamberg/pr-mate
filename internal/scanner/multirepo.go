package scanner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RepoSource represents a repository to scan
type RepoSource struct {
	Address      string // e.g., "github.com/owner/repo" or "owner/repo"
	LocalPath    string // path after cloning
	HasPRMate    bool   // whether .prmate.md exists
	PRMateContent string // content of .prmate.md if exists
}

// MultiRepoResult contains combined results from multiple repos
type MultiRepoResult struct {
	CurrentRepo     *CodebaseContext
	CurrentAnalysis *AnalysisResult
	ExternalRepos   []ExternalRepoData
	MergedRules     []string
}

// ExternalRepoData holds data from an external repo
type ExternalRepoData struct {
	Source       RepoSource
	Context      *CodebaseContext
	Analysis     *AnalysisResult
	Instructions []InstructionFile
	Error        error
}

// MultiRepoScanner scans multiple repositories
type MultiRepoScanner struct {
	scanner      *Scanner
	analyzer     *Analyzer
	instructions *InstructionsReader
	workDir      string
	githubToken  string
}

// NewMultiRepoScanner creates a new multi-repo scanner
func NewMultiRepoScanner(githubToken string) (*MultiRepoScanner, error) {
	workDir := filepath.Join(os.TempDir(), "prmate-scan")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("create work directory: %w", err)
	}

	return &MultiRepoScanner{
		scanner:      NewScanner(),
		analyzer:     NewAnalyzer(),
		instructions: NewInstructionsReader(),
		workDir:      workDir,
		githubToken:  githubToken,
	}, nil
}

// ScanWithExternals scans current repo and any external repos from @scan directive
func (m *MultiRepoScanner) ScanWithExternals(ctx context.Context, currentRepoPath string, externalRepos []string) (*MultiRepoResult, error) {
	result := &MultiRepoResult{
		ExternalRepos: make([]ExternalRepoData, 0),
		MergedRules:   make([]string, 0),
	}

	// Scan current repo
	currentCtx, err := m.scanner.Scan(currentRepoPath)
	if err != nil {
		return nil, fmt.Errorf("scan current repo: %w", err)
	}
	result.CurrentRepo = currentCtx

	// Analyze current repo
	currentAnalysis, err := m.analyzer.Analyze(currentCtx)
	if err != nil {
		return nil, fmt.Errorf("analyze current repo: %w", err)
	}
	result.CurrentAnalysis = currentAnalysis

	// Read current repo instructions
	currentInstructions, _ := m.instructions.ReadInstructions(currentRepoPath)
	currentRules := m.instructions.ExtractRulesFromInstructions(currentInstructions)
	result.MergedRules = append(result.MergedRules, currentRules...)

	// Scan external repos
	for _, repoAddr := range externalRepos {
		externalData := m.scanExternalRepo(ctx, repoAddr)
		result.ExternalRepos = append(result.ExternalRepos, externalData)

		// If external repo has .prmate.md, use its rules directly
		// Otherwise use analyzed patterns
		if externalData.Error == nil {
			externalRules := m.instructions.ExtractRulesFromInstructions(externalData.Instructions)
			result.MergedRules = append(result.MergedRules, externalRules...)
		}
	}

	return result, nil
}

func (m *MultiRepoScanner) scanExternalRepo(ctx context.Context, repoAddr string) ExternalRepoData {
	data := ExternalRepoData{
		Source: RepoSource{
			Address: repoAddr,
		},
	}

	// Normalize repo address
	repoAddr = normalizeRepoAddress(repoAddr)
	repoName := extractRepoName(repoAddr)

	// Clone repo
	localPath := filepath.Join(m.workDir, repoName)
	data.Source.LocalPath = localPath

	if err := m.cloneRepo(ctx, repoAddr, localPath); err != nil {
		data.Error = fmt.Errorf("clone repo: %w", err)
		return data
	}

	// Check for .prmate.md first
	prmatePath := filepath.Join(localPath, ".prmate.md")
	if content, err := os.ReadFile(prmatePath); err == nil {
		data.Source.HasPRMate = true
		data.Source.PRMateContent = string(content)
	}

	// Read all instruction files
	instructions, err := m.instructions.ReadInstructions(localPath)
	if err == nil {
		data.Instructions = instructions
	}

	// If no .prmate.md, do full scan
	if !data.Source.HasPRMate {
		repoCtx, err := m.scanner.Scan(localPath)
		if err != nil {
			data.Error = fmt.Errorf("scan repo: %w", err)
			return data
		}
		data.Context = repoCtx

		analysis, err := m.analyzer.Analyze(repoCtx)
		if err != nil {
			data.Error = fmt.Errorf("analyze repo: %w", err)
			return data
		}
		data.Analysis = analysis
	}

	return data
}

func (m *MultiRepoScanner) cloneRepo(ctx context.Context, repoAddr, localPath string) error {
	// Remove existing directory if present
	_ = os.RemoveAll(localPath)

	// Build clone URL with token
	cloneURL := fmt.Sprintf("https://%s@%s.git", m.githubToken, repoAddr)

	// Use git clone (more reliable than gh for this use case)
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", cloneURL, localPath)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %s: %w", string(output), err)
	}

	return nil
}

// Cleanup removes all cloned repos from temp directory
func (m *MultiRepoScanner) Cleanup() error {
	return os.RemoveAll(m.workDir)
}

func normalizeRepoAddress(addr string) string {
	// Remove https:// prefix if present
	addr = strings.TrimPrefix(addr, "https://")
	addr = strings.TrimPrefix(addr, "http://")

	// Add github.com if not present
	if !strings.HasPrefix(addr, "github.com/") {
		addr = "github.com/" + addr
	}

	// Remove .git suffix
	addr = strings.TrimSuffix(addr, ".git")

	return addr
}

func extractRepoName(addr string) string {
	parts := strings.Split(addr, "/")
	if len(parts) >= 2 {
		// Return owner_repo format to avoid conflicts
		return parts[len(parts)-2] + "_" + parts[len(parts)-1]
	}
	return filepath.Base(addr)
}

// MergeAnalysisResults combines analysis from multiple repos
// Current repo patterns take priority, external patterns fill gaps
func MergeAnalysisResults(current *AnalysisResult, externals []*AnalysisResult) *AnalysisResult {
	merged := &AnalysisResult{
		FolderNaming:      current.FolderNaming,
		FileNaming:        current.FileNaming,
		FolderConventions: current.FolderConventions,
		Abstractions:      current.Abstractions,
		NamingPatterns:    current.NamingPatterns,
		ErrorPatterns:     current.ErrorPatterns,
		TestConventions:   current.TestConventions,
		ImportPatterns:    current.ImportPatterns,
	}

	// Only add external patterns if current doesn't have them
	for _, ext := range externals {
		if ext == nil {
			continue
		}

		// Merge abstractions not in current
		for _, extAbs := range ext.Abstractions {
			found := false
			for _, curAbs := range merged.Abstractions {
				if curAbs.Name == extAbs.Name {
					found = true
					break
				}
			}
			if !found {
				merged.Abstractions = append(merged.Abstractions, extAbs)
			}
		}

		// Merge naming patterns not in current
		for _, extPattern := range ext.NamingPatterns {
			found := false
			for _, curPattern := range merged.NamingPatterns {
				if curPattern.Pattern == extPattern.Pattern {
					found = true
					break
				}
			}
			if !found {
				merged.NamingPatterns = append(merged.NamingPatterns, extPattern)
			}
		}
	}

	return merged
}
