package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanner_Scan(t *testing.T) {
	// Create a temp directory with test files
	tmpDir := t.TempDir()

	// Create folder structure
	dirs := []string{
		"internal/handlers",
		"internal/config",
		"pkg/utils",
		".git",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, d), 0755); err != nil {
			t.Fatalf("create dir: %v", err)
		}
	}

	// Create test files
	files := map[string]string{
		"main.go":                     "package main\n\nfunc main() {}",
		"internal/handlers/health.go": "package handlers\n\nfunc Health() {}",
		"internal/config/config.go":   "package config\n\ntype Config struct{}",
		"pkg/utils/helper.go":         "package utils\n\nfunc Helper() {}",
		".gitignore":                  "*.exe\n",
	}
	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	// Run scanner
	s := NewScanner()
	ctx, err := s.Scan(tmpDir)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	// Verify results
	if ctx.RepoName != filepath.Base(tmpDir) {
		t.Errorf("expected repo name %s, got %s", filepath.Base(tmpDir), ctx.RepoName)
	}

	if len(ctx.Files) != 5 {
		t.Errorf("expected 5 files, got %d", len(ctx.Files))
	}

	if ctx.Extensions[".go"] != 4 {
		t.Errorf("expected 4 .go files, got %d", ctx.Extensions[".go"])
	}

	// .git should be ignored
	for _, f := range ctx.Files {
		if filepath.Base(filepath.Dir(f.Path)) == ".git" {
			t.Errorf("should not include .git files")
		}
	}

	// Check top-level folders
	expectedFolders := map[string]bool{"internal": true, "pkg": true}
	for _, folder := range ctx.TopLevelFolders {
		if !expectedFolders[folder] {
			t.Errorf("unexpected top-level folder: %s", folder)
		}
	}
}

func TestScanner_ExtractGoPackage(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	content := `package mypackage

import "fmt"

func Hello() {
	fmt.Println("Hello")
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	pkg := extractGoPackage(testFile)
	if pkg != "mypackage" {
		t.Errorf("expected package 'mypackage', got '%s'", pkg)
	}
}

func TestScanner_ShouldIgnore(t *testing.T) {
	s := NewScanner()

	tests := []struct {
		path     string
		isDir    bool
		expected bool
	}{
		{".git", true, true},
		{"node_modules", true, true},
		{"vendor", true, true},
		{"src", true, false},
		{"internal", true, false},
		{"file.exe", false, true},
		{"file.go", false, false},
		{"file.dll", false, true},
	}

	for _, tt := range tests {
		result := s.shouldIgnore(tt.path, tt.isDir)
		if result != tt.expected {
			t.Errorf("shouldIgnore(%s, %v) = %v, want %v", tt.path, tt.isDir, result, tt.expected)
		}
	}
}
