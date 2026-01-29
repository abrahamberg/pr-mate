package scan

import (
	"testing"
)

func TestService_CheckForPRMateDirective(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "has @prmate directive",
			content: "# PR Context\n@prmate\nsome content",
			want:    true,
		},
		{
			name:    "no directive",
			content: "# Just a README\nSome content here",
			want:    false,
		},
		{
			name:    "empty content",
			content: "",
			want:    false,
		},
		{
			name:    "directive in comment block",
			content: "<!-- PRMate\n@prmate\n-->\n# Title",
			want:    true,
		},
	}

	// Create a service with nil client (not needed for this test)
	s := &Service{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.CheckForPRMateDirective(tt.content)
			if got != tt.want {
				t.Errorf("CheckForPRMateDirective() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestService_RemoveScanDirectiveFromContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "replaces @scan with @scanned",
			content: "<!-- PRMate\n@scan\nowner/repo\n-->\n# Title",
			want:    "<!-- PRMate\n@scanned\nowner/repo\n-->\n# Title",
		},
		{
			name:    "no @scan",
			content: "# Just content",
			want:    "# Just content",
		},
		{
			name:    "already @scanned",
			content: "<!-- @scanned -->",
			want:    "<!-- @scanned -->",
		},
	}

	s := &Service{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.RemoveScanDirectiveFromContent(tt.content)
			if got != tt.want {
				t.Errorf("RemoveScanDirectiveFromContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScanRequest_Fields(t *testing.T) {
	req := ScanRequest{
		Owner:         "test-owner",
		Repo:          "test-repo",
		PRNumber:      42,
		Branch:        "feature-branch",
		ExternalRepos: []string{"org/other-repo"},
	}

	if req.Owner != "test-owner" {
		t.Errorf("Owner = %v, want test-owner", req.Owner)
	}
	if req.Repo != "test-repo" {
		t.Errorf("Repo = %v, want test-repo", req.Repo)
	}
	if req.PRNumber != 42 {
		t.Errorf("PRNumber = %v, want 42", req.PRNumber)
	}
	if req.Branch != "feature-branch" {
		t.Errorf("Branch = %v, want feature-branch", req.Branch)
	}
	if len(req.ExternalRepos) != 1 || req.ExternalRepos[0] != "org/other-repo" {
		t.Errorf("ExternalRepos = %v, want [org/other-repo]", req.ExternalRepos)
	}
}

func TestScanResult_Fields(t *testing.T) {
	result := ScanResult{
		PRMateContent: "# Generated Content",
		TempFilePath:  "/tmp/prmate-123.md",
		Error:         nil,
	}

	if result.PRMateContent != "# Generated Content" {
		t.Errorf("PRMateContent = %v, want # Generated Content", result.PRMateContent)
	}
	if result.TempFilePath != "/tmp/prmate-123.md" {
		t.Errorf("TempFilePath = %v, want /tmp/prmate-123.md", result.TempFilePath)
	}
	if result.Error != nil {
		t.Errorf("Error = %v, want nil", result.Error)
	}
}
