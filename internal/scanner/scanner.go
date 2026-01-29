package scanner

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FileInfo represents a file in the codebase
type FileInfo struct {
	Path      string
	Name      string
	Extension string
	IsDir     bool
	Size      int64
	Package   string // For Go files, the package name
}

// FolderInfo represents a folder with its contents
type FolderInfo struct {
	Path     string
	Name     string
	Files    []FileInfo
	Children []FolderInfo
	Depth    int
}

// CodebaseContext holds all extracted information about a codebase
type CodebaseContext struct {
	RootPath        string
	RepoName        string
	Files           []FileInfo
	FolderTree      FolderInfo
	Extensions      map[string]int         // extension -> count
	Packages        map[string][]string    // package name -> file paths
	FoldersByDepth  map[int][]string       // depth -> folder paths
	TopLevelFolders []string               // immediate children of root
	IgnoredPaths    []string               // paths that were ignored
}

// Scanner scans a codebase and extracts structure information
type Scanner struct {
	ignoredDirs  map[string]bool
	ignoredExts  map[string]bool
	gitignorePatterns []string
}

// NewScanner creates a new scanner with default ignore patterns
func NewScanner() *Scanner {
	return &Scanner{
		ignoredDirs: map[string]bool{
			".git":         true,
			"node_modules": true,
			"vendor":       true,
			".idea":        true,
			".vscode":      true,
			"__pycache__":  true,
			".pytest_cache": true,
			"dist":         true,
			"build":        true,
			".next":        true,
			"coverage":     true,
		},
		ignoredExts: map[string]bool{
			".exe":  true,
			".dll":  true,
			".so":   true,
			".dylib": true,
			".o":    true,
			".a":    true,
		},
	}
}

// Scan scans a repository and returns its context
func (s *Scanner) Scan(repoPath string) (*CodebaseContext, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	}

	ctx := &CodebaseContext{
		RootPath:        absPath,
		RepoName:        filepath.Base(absPath),
		Files:           make([]FileInfo, 0),
		Extensions:      make(map[string]int),
		Packages:        make(map[string][]string),
		FoldersByDepth:  make(map[int][]string),
		TopLevelFolders: make([]string, 0),
		IgnoredPaths:    make([]string, 0),
	}

	// Load gitignore if exists
	s.loadGitignore(absPath)

	// Scan the directory tree
	ctx.FolderTree, err = s.scanDirectory(absPath, 0, ctx)
	if err != nil {
		return nil, err
	}

	// Extract top-level folders
	for _, child := range ctx.FolderTree.Children {
		ctx.TopLevelFolders = append(ctx.TopLevelFolders, child.Name)
	}

	return ctx, nil
}

func (s *Scanner) loadGitignore(repoPath string) {
	gitignorePath := filepath.Join(repoPath, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		return
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			s.gitignorePatterns = append(s.gitignorePatterns, line)
		}
	}
}

func (s *Scanner) shouldIgnore(path string, isDir bool) bool {
	name := filepath.Base(path)

	// Check ignored directories
	if isDir && s.ignoredDirs[name] {
		return true
	}

	// Check ignored extensions
	ext := filepath.Ext(name)
	if s.ignoredExts[ext] {
		return true
	}

	// Check gitignore patterns (simplified matching)
	for _, pattern := range s.gitignorePatterns {
		// Handle directory patterns
		if strings.HasSuffix(pattern, "/") {
			if isDir && strings.TrimSuffix(pattern, "/") == name {
				return true
			}
			continue
		}
		// Handle file patterns
		matched, _ := filepath.Match(pattern, name)
		if matched {
			return true
		}
	}

	return false
}

func (s *Scanner) scanDirectory(dirPath string, depth int, ctx *CodebaseContext) (FolderInfo, error) {
	folder := FolderInfo{
		Path:     dirPath,
		Name:     filepath.Base(dirPath),
		Depth:    depth,
		Files:    make([]FileInfo, 0),
		Children: make([]FolderInfo, 0),
	}

	ctx.FoldersByDepth[depth] = append(ctx.FoldersByDepth[depth], dirPath)

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return folder, err
	}

	for _, entry := range entries {
		entryPath := filepath.Join(dirPath, entry.Name())
		isDir := entry.IsDir()

		if s.shouldIgnore(entryPath, isDir) {
			ctx.IgnoredPaths = append(ctx.IgnoredPaths, entryPath)
			continue
		}

		if isDir {
			childFolder, err := s.scanDirectory(entryPath, depth+1, ctx)
			if err != nil {
				continue
			}
			folder.Children = append(folder.Children, childFolder)
		} else {
			info, err := entry.Info()
			if err != nil {
				continue
			}

			fileInfo := s.extractFileInfo(entryPath, info)
			folder.Files = append(folder.Files, fileInfo)
			ctx.Files = append(ctx.Files, fileInfo)

			// Track extensions
			if fileInfo.Extension != "" {
				ctx.Extensions[fileInfo.Extension]++
			}

			// Track packages for Go files
			if fileInfo.Package != "" {
				ctx.Packages[fileInfo.Package] = append(ctx.Packages[fileInfo.Package], entryPath)
			}
		}
	}

	return folder, nil
}

func (s *Scanner) extractFileInfo(path string, info fs.FileInfo) FileInfo {
	ext := filepath.Ext(path)
	fi := FileInfo{
		Path:      path,
		Name:      info.Name(),
		Extension: ext,
		IsDir:     false,
		Size:      info.Size(),
	}

	// Extract package name for Go files
	if ext == ".go" {
		fi.Package = extractGoPackage(path)
	}

	return fi
}

func extractGoPackage(filePath string) string {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}
