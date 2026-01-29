package github

import (
	"testing"
)

func TestParseRepoFullName(t *testing.T) {
	tests := []struct {
		name      string
		fullName  string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "valid repo name",
			fullName:  "owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "org with dashes",
			fullName:  "my-org/my-repo",
			wantOwner: "my-org",
			wantRepo:  "my-repo",
			wantErr:   false,
		},
		{
			name:      "repo with multiple slashes",
			fullName:  "owner/repo/extra",
			wantOwner: "owner",
			wantRepo:  "repo/extra",
			wantErr:   false,
		},
		{
			name:     "missing slash",
			fullName: "noslash",
			wantErr:  true,
		},
		{
			name:     "empty string",
			fullName: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := ParseRepoFullName(tt.fullName)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseRepoFullName(%q) expected error, got nil", tt.fullName)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseRepoFullName(%q) unexpected error: %v", tt.fullName, err)
				return
			}

			if owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
			}
		})
	}
}

func TestClient_CloneURL(t *testing.T) {
	client := &Client{token: "test-token"}

	url := client.CloneURL("myowner", "myrepo")
	expected := "https://test-token@github.com/myowner/myrepo.git"

	if url != expected {
		t.Errorf("CloneURL() = %q, want %q", url, expected)
	}
}

func TestClient_GetToken(t *testing.T) {
	token := "my-secret-token"
	client := &Client{token: token}

	if got := client.GetToken(); got != token {
		t.Errorf("GetToken() = %q, want %q", got, token)
	}
}

func TestNewClient_WithToken(t *testing.T) {
	token := "explicit-token"
	client := NewClient(token)

	if client.token != token {
		t.Errorf("token = %q, want %q", client.token, token)
	}
	if client.client == nil {
		t.Error("client.client should not be nil")
	}
}

func TestPRFile_Fields(t *testing.T) {
	f := PRFile{
		Filename:  "src/main.go",
		Status:    "modified",
		Additions: 10,
		Deletions: 5,
		Patch:     "@@ -1,5 +1,10 @@",
	}

	if f.Filename != "src/main.go" {
		t.Errorf("Filename = %q, want src/main.go", f.Filename)
	}
	if f.Status != "modified" {
		t.Errorf("Status = %q, want modified", f.Status)
	}
	if f.Additions != 10 {
		t.Errorf("Additions = %d, want 10", f.Additions)
	}
	if f.Deletions != 5 {
		t.Errorf("Deletions = %d, want 5", f.Deletions)
	}
	if f.Patch != "@@ -1,5 +1,10 @@" {
		t.Errorf("Patch = %q, want @@ -1,5 +1,10 @@", f.Patch)
	}
}
