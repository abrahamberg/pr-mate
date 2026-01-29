//go:build ignore

package main

import (
	"fmt"
	"log"
	"os"

	prcontext "prmate/internal/context"
	"prmate/internal/scanner"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run tools/scan_test.go <repo_path>")
	}

	repoPath := os.Args[1]

	// Create scanner
	s := scanner.NewScanner()
	ctx, err := s.Scan(repoPath)
	if err != nil {
		log.Fatalf("Scan failed: %v", err)
	}

	fmt.Printf("Scanned %s\n", ctx.RepoName)
	fmt.Printf("Files: %d\n", len(ctx.Files))
	fmt.Printf("Extensions: %v\n", ctx.Extensions)
	fmt.Printf("Top-level folders: %v\n", ctx.TopLevelFolders)
	fmt.Println()

	// Analyze
	analyzer := scanner.NewAnalyzer()
	analysis, err := analyzer.Analyze(ctx)
	if err != nil {
		log.Fatalf("Analysis failed: %v", err)
	}

	fmt.Printf("Folder naming: %s\n", analysis.FolderNaming)
	fmt.Printf("File naming: %s\n", analysis.FileNaming)
	fmt.Printf("Abstractions: %d\n", len(analysis.Abstractions))
	fmt.Printf("Error patterns: %d\n", len(analysis.ErrorPatterns))
	fmt.Println()

	// Read instructions
	reader := scanner.NewInstructionsReader()
	instructions, _ := reader.ReadInstructions(repoPath)
	fmt.Printf("Instruction files found: %d\n", len(instructions))
	for _, inst := range instructions {
		fmt.Printf("  - %s (%s)\n", inst.Path, inst.Type)
	}
	fmt.Println()

	// Generate context
	multiResult := &scanner.MultiRepoResult{
		CurrentRepo:     ctx,
		CurrentAnalysis: analysis,
		ExternalRepos:   nil,
		MergedRules:     reader.ExtractRulesFromInstructions(instructions),
	}

	generator := prcontext.NewGenerator()
	content := generator.Generate(multiResult)

	fmt.Println("=== Generated .prmate.md ===")
	fmt.Println(content)
}
