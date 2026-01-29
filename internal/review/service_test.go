package review

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	ghclient "prmate/internal/github"
)

// Mock implementations

type mockGitHubClient struct {
	pullRequest     *ghclient.PullRequest
	prFiles         []ghclient.PRFile
	fileContents    map[string]string
	prComments      []string
	reviewComments  []ghclient.ReviewComment
	postedReviews   []mockPostedReview
	postedComments  []string
}

type mockPostedReview struct {
	commitID string
	event    string
	body     string
	comments []ghclient.DraftReviewComment
}

func (m *mockGitHubClient) GetPullRequest(ctx context.Context, owner, repo string, prNumber int) (*ghclient.PullRequest, error) {
	return m.pullRequest, nil
}

func (m *mockGitHubClient) GetPRFiles(ctx context.Context, owner, repo string, prNumber int) ([]ghclient.PRFile, error) {
	return m.prFiles, nil
}

func (m *mockGitHubClient) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	if content, ok := m.fileContents[path]; ok {
		return content, nil
	}
	return "", nil
}

func (m *mockGitHubClient) ListPRComments(ctx context.Context, owner, repo string, prNumber int) ([]string, error) {
	return m.prComments, nil
}

func (m *mockGitHubClient) ListReviewComments(ctx context.Context, owner, repo string, prNumber int) ([]ghclient.ReviewComment, error) {
	return m.reviewComments, nil
}

func (m *mockGitHubClient) CreatePullRequestReview(ctx context.Context, owner, repo string, prNumber int, commitID string, event string, body string, comments []ghclient.DraftReviewComment) error {
	m.postedReviews = append(m.postedReviews, mockPostedReview{
		commitID: commitID,
		event:    event,
		body:     body,
		comments: comments,
	})
	return nil
}

func (m *mockGitHubClient) CreatePRComment(ctx context.Context, owner, repo string, prNumber int, body string) error {
	m.postedComments = append(m.postedComments, body)
	return nil
}

type mockLLMProvider struct {
	response string
}

func (m *mockLLMProvider) GenerateText(prompt string) (string, error) {
	return m.response, nil
}

// Tests

func TestParsePatch(t *testing.T) {
	tests := []struct {
		name          string
		patch         string
		expectedHunks int
		expectedLines []int // expected new line numbers from added lines
	}{
		{
			name:          "empty patch",
			patch:         "",
			expectedHunks: 0,
			expectedLines: nil,
		},
		{
			name: "single hunk with additions",
			patch: `@@ -1,3 +1,5 @@
 package main
+
+import "fmt"
 
 func main() {`,
			expectedHunks: 1,
			expectedLines: []int{2, 3},
		},
		{
			name: "multiple hunks",
			patch: `@@ -1,3 +1,4 @@
 package main
+import "fmt"
 
 func main() {
@@ -10,3 +11,4 @@
 }
+
+func helper() {}`,
			expectedHunks: 2,
			expectedLines: []int{2, 12, 13},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hunks := ghclient.ParsePatch(tt.patch)
			if len(hunks) != tt.expectedHunks {
				t.Errorf("expected %d hunks, got %d", tt.expectedHunks, len(hunks))
			}

			newLines := ghclient.GetNewLineNumbers(tt.patch)
			if len(newLines) != len(tt.expectedLines) {
				t.Errorf("expected %d new lines, got %d", len(tt.expectedLines), len(newLines))
			}

			for i, expected := range tt.expectedLines {
				if i < len(newLines) && newLines[i] != expected {
					t.Errorf("line %d: expected %d, got %d", i, expected, newLines[i])
				}
			}
		})
	}
}

func TestParseSummaryFromComment(t *testing.T) {
	summary := ReviewSummary{
		Version:         "1.0",
		LastReviewedAt:  time.Now(),
		HeadSHA:         "abc123def456",
		FilesScanned:    []FileReviewStatus{{Path: "main.go", LastSHA: "abc123", Violations: 1}},
		RulesApplied:    5,
		ViolationsFound: 2,
	}

	summaryJSON, _ := json.Marshal(summary)
	comment := `<!-- prmate-review-summary:abc123def456 -->
## 📊 PRMate Review Summary

| Metric | Value |
|--------|-------|
| Files Reviewed | 1 |

<!-- prmate-data:` + string(summaryJSON) + ` -->`

	parsed, err := parseSummaryFromComment(comment)
	if err != nil {
		t.Fatalf("failed to parse summary: %v", err)
	}

	if parsed.HeadSHA != summary.HeadSHA {
		t.Errorf("expected HeadSHA %s, got %s", summary.HeadSHA, parsed.HeadSHA)
	}

	if parsed.ViolationsFound != summary.ViolationsFound {
		t.Errorf("expected ViolationsFound %d, got %d", summary.ViolationsFound, parsed.ViolationsFound)
	}
}

func TestExtractChecklistItems(t *testing.T) {
	content := `
Some intro text.

- [ ] Check error handling patterns
- [x] Verify naming conventions
- [ ] Add tests
- Regular bullet point
`

	items := extractChecklistItems(content)

	if len(items) != 3 {
		t.Errorf("expected 3 checklist items, got %d", len(items))
	}

	expectedItems := []string{
		"Check error handling patterns",
		"Verify naming conventions",
		"Add tests",
	}

	for i, expected := range expectedItems {
		if i < len(items) && items[i] != expected {
			t.Errorf("item %d: expected %q, got %q", i, expected, items[i])
		}
	}
}

func TestFilterFilesToReview(t *testing.T) {
	service := &Service{}

	files := []ghclient.PRFile{
		{Filename: "main.go", Status: "modified", Patch: "+new code"},
		{Filename: "unchanged.go", Status: "modified", Patch: ""},
		{Filename: "new.go", Status: "added", Patch: "+content"},
	}

	previousSummary := &ReviewSummary{
		HeadSHA: "abc123",
		FilesScanned: []FileReviewStatus{
			{Path: "main.go", LastSHA: "abc123"},
			{Path: "unchanged.go", LastSHA: "abc123"},
		},
	}

	// With current SHA different from last reviewed
	filtered := service.filterFilesToReview(files, previousSummary, "def456")
	if len(filtered) != 3 {
		t.Errorf("expected 3 files when SHA changed, got %d", len(filtered))
	}

	// Test with nil summary (first review)
	filtered = service.filterFilesToReview(files, nil, "abc123")
	if len(filtered) != 3 {
		t.Errorf("expected 3 files on first review, got %d", len(filtered))
	}
}

func TestReviewPR_NoRules(t *testing.T) {
	ghMock := &mockGitHubClient{
		fileContents: map[string]string{
			".prmate.md": "# PRMate Context\n\nEmpty file with no rules.",
		},
		prFiles: []ghclient.PRFile{
			{Filename: "main.go", Status: "modified", Patch: "+code"},
		},
	}

	llmMock := &mockLLMProvider{
		response: `{"violations": []}`,
	}

	svc := NewService(ghMock, llmMock)

	result, err := svc.ReviewPR(context.Background(), ReviewRequest{
		Owner:    "test",
		Repo:     "repo",
		PRNumber: 1,
		HeadSHA:  "abc123def456789",
		HeadRef:  "feature-branch",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ViolationsFound != 0 {
		t.Errorf("expected 0 violations, got %d", result.ViolationsFound)
	}
}

func TestReviewPR_WithViolations(t *testing.T) {
	prmateMD := `# PRMate Context

## Review Checklist
- [ ] All errors must be wrapped with context

## Learned Rules
- Use fmt.Errorf with %w for error wrapping
`

	ghMock := &mockGitHubClient{
		fileContents: map[string]string{
			".prmate.md": prmateMD,
			"handler.go": "package main\n\nfunc foo() error {\n\treturn err\n}",
		},
		prFiles: []ghclient.PRFile{
			{Filename: "handler.go", Status: "modified", Additions: 1, Deletions: 0, Patch: "@@ -3,0 +4 @@\n+\treturn err"},
		},
	}

	llmMock := &mockLLMProvider{
		response: `{"violations": [{"line": 4, "rule": "Error Handling", "message": "Error not wrapped with context", "severity": "warning"}]}`,
	}

	svc := NewService(ghMock, llmMock)

	result, err := svc.ReviewPR(context.Background(), ReviewRequest{
		Owner:    "test",
		Repo:     "repo",
		PRNumber: 1,
		HeadSHA:  "abc123def456789",
		HeadRef:  "feature-branch",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ViolationsFound != 1 {
		t.Errorf("expected 1 violation, got %d", result.ViolationsFound)
	}

	if result.CommentsPosted != 1 {
		t.Errorf("expected 1 comment posted, got %d", result.CommentsPosted)
	}

	if len(ghMock.postedReviews) != 1 {
		t.Fatalf("expected 1 review posted, got %d", len(ghMock.postedReviews))
	}

	review := ghMock.postedReviews[0]
	if review.event != "COMMENT" {
		t.Errorf("expected COMMENT event for warning, got %s", review.event)
	}

	if len(review.comments) != 1 {
		t.Errorf("expected 1 inline comment, got %d", len(review.comments))
	}
}

func TestBuildAnalysisPrompt(t *testing.T) {
	svc := &Service{}

	prompt := svc.buildAnalysisPrompt(
		"main.go",
		"package main\n\nfunc main() {}",
		"@@ -1,3 +1,4 @@\n+import \"fmt\"",
		[]string{"Use fmt.Errorf for errors"},
		[]string{"Check naming conventions"},
		"## Structure\nClean architecture",
		"### internal/types.go\n```go\ntype Service interface {}\n```",
	)

	// Check key elements are in the prompt
	if !contains(prompt, "main.go") {
		t.Error("prompt should contain filename")
	}
	if !contains(prompt, "Use fmt.Errorf for errors") {
		t.Error("prompt should contain rules")
	}
	if !contains(prompt, "Check naming conventions") {
		t.Error("prompt should contain checklist")
	}
	if !contains(prompt, "Clean architecture") {
		t.Error("prompt should contain codebase info")
	}
	if !contains(prompt, "internal/types.go") {
		t.Error("prompt should contain dependency context")
	}
	if !contains(prompt, "JSON") {
		t.Error("prompt should request JSON response")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
