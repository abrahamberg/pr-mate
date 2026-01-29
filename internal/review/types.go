package review

import "time"

// ReviewRequest contains parameters for reviewing a PR
type ReviewRequest struct {
	Owner    string
	Repo     string
	PRNumber int
	HeadSHA  string
	HeadRef  string
	BaseSHA  string
}

// ReviewResult contains the outcome of a PR review
type ReviewResult struct {
	FilesReviewed   int
	CommentsPosted  int
	ViolationsFound int
	SummaryPosted   bool
	ReviewedCommit  string
}

// FileViolation represents a rule violation found in a file
type FileViolation struct {
	Path         string
	Line         int
	Rule         string
	Message      string
	Severity     string // "error", "warning", "suggestion"
	CodeSnippet  string
}

// ReviewSummary is the tracking data stored in PR comments
type ReviewSummary struct {
	Version         string              `json:"version"`
	LastReviewedAt  time.Time           `json:"last_reviewed_at"`
	HeadSHA         string              `json:"head_sha"`
	FilesScanned    []FileReviewStatus  `json:"files_scanned"`
	RulesApplied    int                 `json:"rules_applied"`
	ViolationsFound int                 `json:"violations_found"`
}

// FileReviewStatus tracks review state per file
type FileReviewStatus struct {
	Path        string `json:"path"`
	LastSHA     string `json:"last_sha"`
	Violations  int    `json:"violations"`
	ReviewedAt  string `json:"reviewed_at"`
}

// LLMAnalysisRequest is the input for LLM file analysis
type LLMAnalysisRequest struct {
	FilePath      string
	FileContent   string
	Patch         string
	Rules         []string
	Checklist     []string
	CodebaseInfo  string
}

// LLMAnalysisResponse is the expected output from LLM analysis
type LLMAnalysisResponse struct {
	Violations []LLMViolation `json:"violations"`
	Summary    string         `json:"summary"`
}

// LLMViolation is a single violation detected by the LLM
type LLMViolation struct {
	Line     int    `json:"line"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
	Fix      string `json:"fix,omitempty"`
}
