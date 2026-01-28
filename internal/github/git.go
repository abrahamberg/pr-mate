package github

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

type Service struct {
	files []string
}

func NewService() *Service {
	return &Service{
		files: []string{},
	}
}

func getPath() string {
	homeDir, _ := os.UserHomeDir()
	workspace := filepath.Join(homeDir, "workspace")
	return workspace
}

// run gh repo clone repoName in workspace folder
func (s *Service) Clone(repoName string) {
	workspace := getPath()
	//create workspace folder if not exists
	if _, err := os.Stat(workspace); os.IsNotExist(err) {
		_ = os.MkdirAll(workspace, os.ModePerm)
	}
	_ = os.Chdir(workspace)
	// Here you would add the logic to run the command
	// For example, using os/exec to run "gh repo clone repoName"
	cmd := exec.Command("gh", "repo", "clone", repoName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Command execution failed: %s", err)
	}

	// After cloning, list files in the cloned repo
	files, err := os.ReadDir(filepath.Join(workspace, repoName))
	if err != nil {
		log.Fatalf("Failed to list files: %s", err)
	}
	for _, file := range files {
		s.files = append(s.files, file.Name())
	}
}
