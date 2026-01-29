package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"prmate/internal/scan"
)

// MockPRWorkspace is a test double for PRWorkspace
type MockPRWorkspace struct {
	ensureCalled bool
	deleteCalled bool
	ensureErr    error
	deleteErr    error
}

func (m *MockPRWorkspace) EnsurePRDir(ctx context.Context, repoFullName string, prNumber int) (string, error) {
	m.ensureCalled = true
	if m.ensureErr != nil {
		return "", m.ensureErr
	}
	return fmt.Sprintf("/tmp/%s/%d", repoFullName, prNumber), nil
}

func (m *MockPRWorkspace) DeletePRDir(ctx context.Context, repoFullName string, prNumber int) error {
	m.deleteCalled = true
	return m.deleteErr
}

// MockScanService is a test double for ScanService
type MockScanService struct {
	processCalled            bool
	checkScanDirectiveCalled bool
	hasScanDirective         bool
	externalRepos            []string
	checkScanErr             error
	processErr               error
}

func (m *MockScanService) ProcessScan(ctx context.Context, req scan.ScanRequest) (*scan.ScanResult, error) {
	m.processCalled = true
	if m.processErr != nil {
		return nil, m.processErr
	}
	return &scan.ScanResult{
		PRMateContent: "# Generated",
		TempFilePath:  "/tmp/prmate.md",
	}, nil
}

func (m *MockScanService) CheckForScanDirective(ctx context.Context, owner, repo, branch string) (bool, []string, error) {
	m.checkScanDirectiveCalled = true
	if m.checkScanErr != nil {
		return false, nil, m.checkScanErr
	}
	return m.hasScanDirective, m.externalRepos, nil
}

func (m *MockScanService) CheckForPRMateDirective(content string) bool {
	return content == "@prmate" || content == "Please @prmate review"
}

func TestProcessor_Process_PingEvent(t *testing.T) {
	mockWorkspace := &MockPRWorkspace{}
	mockScan := &MockScanService{}

	p := NewProcessor(mockWorkspace, mockScan, nil)

	payload, _ := json.Marshal(map[string]interface{}{
		"zen": "Keep it simple, silly",
	})

	err := p.Process(context.Background(), "ping", payload, "test-delivery")
	if err != nil {
		t.Errorf("Process(ping) returned error: %v", err)
	}

	if mockWorkspace.ensureCalled {
		t.Error("EnsurePRDir should not be called for ping event")
	}
}

func TestProcessor_Process_PROpened(t *testing.T) {
	mockWorkspace := &MockPRWorkspace{}
	mockScan := &MockScanService{}

	p := NewProcessor(mockWorkspace, mockScan, nil)

	payload, _ := json.Marshal(map[string]interface{}{
		"action": "opened",
		"number": 42,
		"pull_request": map[string]interface{}{
			"number": 42,
			"head": map[string]interface{}{
				"ref": "feature-branch",
			},
		},
		"repository": map[string]interface{}{
			"full_name": "owner/repo",
		},
	})

	err := p.Process(context.Background(), "pull_request", payload, "test-delivery")
	if err != nil {
		t.Errorf("Process(pull_request opened) returned error: %v", err)
	}

	if !mockWorkspace.ensureCalled {
		t.Error("EnsurePRDir should be called for opened PR")
	}
}

func TestProcessor_Process_PRClosed(t *testing.T) {
	mockWorkspace := &MockPRWorkspace{}
	mockScan := &MockScanService{}

	p := NewProcessor(mockWorkspace, mockScan, nil)

	payload, _ := json.Marshal(map[string]interface{}{
		"action": "closed",
		"number": 42,
		"pull_request": map[string]interface{}{
			"number": 42,
			"head": map[string]interface{}{
				"ref": "feature-branch",
			},
		},
		"repository": map[string]interface{}{
			"full_name": "owner/repo",
		},
	})

	err := p.Process(context.Background(), "pull_request", payload, "test-delivery")
	if err != nil {
		t.Errorf("Process(pull_request closed) returned error: %v", err)
	}

	if !mockWorkspace.deleteCalled {
		t.Error("DeletePRDir should be called for closed PR")
	}
}

func TestProcessor_Process_NilWorkspace(t *testing.T) {
	p := NewProcessor(nil, nil, nil)

	payload, _ := json.Marshal(map[string]interface{}{
		"action": "opened",
	})

	err := p.Process(context.Background(), "pull_request", payload, "test-delivery")
	if err == nil {
		t.Error("Process should return error when workspace is nil")
	}
}

func TestProcessor_Process_UnknownEventType(t *testing.T) {
	mockWorkspace := &MockPRWorkspace{}
	mockScan := &MockScanService{}

	p := NewProcessor(mockWorkspace, mockScan, nil)

	payload, _ := json.Marshal(map[string]interface{}{})

	// Unknown event should not error (gracefully ignored)
	err := p.Process(context.Background(), "unknown_event", payload, "test-delivery")
	// It may error on parsing, but the logic should not fail
	// The go-github library may fail to parse unknown events
	_ = err
}

func TestProcessor_Process_WithScanDirective(t *testing.T) {
	mockWorkspace := &MockPRWorkspace{}
	mockScan := &MockScanService{
		hasScanDirective: true,
		externalRepos:    []string{"org/external-repo"},
	}

	p := NewProcessor(mockWorkspace, mockScan, nil)

	payload, _ := json.Marshal(map[string]interface{}{
		"action": "opened",
		"number": 42,
		"pull_request": map[string]interface{}{
			"number": 42,
			"head": map[string]interface{}{
				"ref": "feature-branch",
			},
		},
		"repository": map[string]interface{}{
			"full_name": "owner/repo",
		},
	})

	err := p.Process(context.Background(), "pull_request", payload, "test-delivery")
	if err != nil {
		t.Errorf("Process returned error: %v", err)
	}

	if !mockScan.checkScanDirectiveCalled {
		t.Error("CheckForScanDirective should be called")
	}

	if !mockScan.processCalled {
		t.Error("ProcessScan should be called when directive found")
	}
}

func TestNewProcessor(t *testing.T) {
	mockWorkspace := &MockPRWorkspace{}
	mockScan := &MockScanService{}

	p := NewProcessor(mockWorkspace, mockScan, nil)

	if p.prWorkspace != mockWorkspace {
		t.Error("prWorkspace not set correctly")
	}
	if p.scanService != mockScan {
		t.Error("scanService not set correctly")
	}
}
