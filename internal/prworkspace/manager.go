package prworkspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const sentinelFileName = ".prmate-workdir"

type Manager struct {
	baseDir string

	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func NewManager(baseDir string) *Manager {
	return &Manager{baseDir: baseDir, locks: make(map[string]*sync.Mutex)}
}

func (m *Manager) EnsurePRDir(ctx context.Context, repoFullName string, prNumber int) (string, error) {
	_ = ctx
	if prNumber <= 0 {
		return "", fmt.Errorf("invalid pr number: %d", prNumber)
	}

	prDir, key, err := m.prDirPath(repoFullName, prNumber)
	if err != nil {
		return "", err
	}

	lock := m.lockFor(key)
	lock.Lock()
	defer lock.Unlock()

	if err := os.MkdirAll(prDir, 0o755); err != nil {
		return "", fmt.Errorf("create pr workspace dir: %w", err)
	}

	sentinelPath := filepath.Join(prDir, sentinelFileName)
	if err := writeSentinelIfMissing(sentinelPath); err != nil {
		return "", err
	}

	return prDir, nil
}

func (m *Manager) DeletePRDir(ctx context.Context, repoFullName string, prNumber int) error {
	_ = ctx
	if prNumber <= 0 {
		return fmt.Errorf("invalid pr number: %d", prNumber)
	}

	prDir, key, err := m.prDirPath(repoFullName, prNumber)
	if err != nil {
		return err
	}

	lock := m.lockFor(key)
	lock.Lock()
	defer lock.Unlock()

	if _, err := os.Stat(prDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat pr workspace dir: %w", err)
	}

	if err := m.validateSafeDelete(prDir); err != nil {
		return err
	}

	sentinelPath := filepath.Join(prDir, sentinelFileName)
	if _, err := os.Stat(sentinelPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("refusing to delete %q: missing %s", prDir, sentinelFileName)
		}
		return fmt.Errorf("stat sentinel: %w", err)
	}

	if err := os.RemoveAll(prDir); err != nil {
		return fmt.Errorf("delete pr workspace dir: %w", err)
	}

	return nil
}

func (m *Manager) lockFor(key string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()

	lock, ok := m.locks[key]
	if !ok {
		lock = &sync.Mutex{}
		m.locks[key] = lock
	}

	return lock
}

func (m *Manager) prDirPath(repoFullName string, prNumber int) (dir string, key string, err error) {
	baseDir, err := normalizeBaseDir(m.baseDir)
	if err != nil {
		return "", "", err
	}

	repoPath, repoKey, err := sanitizeRepoFullName(repoFullName)
	if err != nil {
		return "", "", err
	}

	key = fmt.Sprintf("%s#%d", repoKey, prNumber)
	dir = filepath.Join(baseDir, repoPath, fmt.Sprintf("pr-%d", prNumber))
	return dir, key, nil
}

func normalizeBaseDir(baseDir string) (string, error) {
	if strings.TrimSpace(baseDir) == "" {
		return "", errors.New("work base dir is empty")
	}

	abs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("abs work base dir: %w", err)
	}

	abs = filepath.Clean(abs)
	if abs == string(filepath.Separator) {
		return "", errors.New("work base dir cannot be filesystem root")
	}

	return abs, nil
}

func sanitizeRepoFullName(repoFullName string) (repoPath string, repoKey string, err error) {
	repoFullName = strings.TrimSpace(repoFullName)
	parts := strings.Split(repoFullName, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo full name %q", repoFullName)
	}

	owner := sanitizePathSegment(parts[0])
	repo := sanitizePathSegment(parts[1])
	if owner == "" || repo == "" || owner == "." || owner == ".." || repo == "." || repo == ".." {
		return "", "", fmt.Errorf("invalid repo full name %q", repoFullName)
	}

	repoPath = filepath.Join(owner, repo)
	repoKey = owner + "/" + repo
	return repoPath, repoKey, nil
}

func sanitizePathSegment(seg string) string {
	seg = strings.TrimSpace(seg)
	seg = strings.ReplaceAll(seg, "\\", "_")
	seg = strings.ReplaceAll(seg, "/", "_")
	seg = strings.ReplaceAll(seg, "\x00", "_")

	b := strings.Builder{}
	b.Grow(len(seg))
	for _, r := range seg {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

func (m *Manager) validateSafeDelete(targetDir string) error {
	baseDir, err := normalizeBaseDir(m.baseDir)
	if err != nil {
		return err
	}

	baseReal, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		return fmt.Errorf("resolve work base dir: %w", err)
	}
	baseReal = filepath.Clean(baseReal)

	targetReal, err := filepath.EvalSymlinks(targetDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("resolve pr workspace dir: %w", err)
	}
	targetReal = filepath.Clean(targetReal)

	if targetReal == baseReal {
		return fmt.Errorf("refusing to delete base dir %q", baseReal)
	}

	rel, err := filepath.Rel(baseReal, targetReal)
	if err != nil {
		return fmt.Errorf("rel path: %w", err)
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return fmt.Errorf("refusing to delete %q: outside base dir %q", targetReal, baseReal)
	}

	return nil
}

func writeSentinelIfMissing(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err == nil {
		_ = f.Close()
		return nil
	}
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	return fmt.Errorf("create sentinel file: %w", err)
}
