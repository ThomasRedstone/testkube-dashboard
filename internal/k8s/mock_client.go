package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/testkube/dashboard/internal/app"
)

type MockK8sService struct {
}

func NewMockK8sService() *MockK8sService {
	return &MockK8sService{}
}

func (s *MockK8sService) ListTests(ctx context.Context, namespace string) ([]app.Test, error) {
	// Return some dummy data
	return []app.Test{
		{
			Name:      "api-sanity-check",
			Namespace: "testkube",
			Type:      "curl/test",
			Labels:    map[string]string{"env": "staging"},
			Created:   time.Now().Add(-24 * time.Hour),
		},
		{
			Name:      "frontend-e2e",
			Namespace: "testkube",
			Type:      "cypress/project",
			Labels:    map[string]string{"env": "production"},
			Created:   time.Now().Add(-48 * time.Hour),
		},
		{
			Name:      "load-test-checkout",
			Namespace: "testkube",
			Type:      "k6/script",
			Labels:    map[string]string{"type": "performance"},
			Created:   time.Now().Add(-72 * time.Hour),
		},
	}, nil
}

func (s *MockK8sService) GetTest(ctx context.Context, namespace, name string) (*app.Test, error) {
	// In a real mock we might search the list, but for now just return a dummy
	return &app.Test{
		Name:      name,
		Namespace: namespace,
		Type:      "curl/test",
		Labels:    map[string]string{"env": "dev"},
		Created:   time.Now(),
	}, nil
}

func (s *MockK8sService) ListExecutions(ctx context.Context, namespace, testName string) ([]app.TestExecution, error) {
	return []app.TestExecution{
		{ID: "exec-1", TestName: testName, Status: "passed", StartTime: time.Now().Add(-1 * time.Hour), EndTime: time.Now().Add(-59 * time.Minute)},
		{ID: "exec-2", TestName: testName, Status: "failed", StartTime: time.Now().Add(-2 * time.Hour), EndTime: time.Now().Add(-119 * time.Minute)},
	}, nil
}

func (s *MockK8sService) GetExecutionLogs(ctx context.Context, namespace, executionID string) (string, error) {
	return fmt.Sprintf("Logs for execution %s\nStep 1: Init...\nStep 2: Run...\nStep 3: Done.", executionID), nil
}
