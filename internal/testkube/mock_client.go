package testkube

import (
	"fmt"
	"time"
)

type MockClient struct {
	executions []Execution
	workflows  []Workflow
}

func NewMockClient() *MockClient {
	c := &MockClient{}
	c.generateMockData()
	return c
}

func (c *MockClient) generateMockData() {
	// Generate some workflows
	c.workflows = []Workflow{
		{
			Name: "frontend-e2e", Namespace: "testkube", Type: "playwright", Created: time.Now().Add(-30 * 24 * time.Hour),
			LastRun: time.Now().Add(-1 * time.Hour), LastStatus: "passed", PassRateLast7d: 95,
		},
		{
			Name: "backend-integration", Namespace: "testkube", Type: "vitest", Created: time.Now().Add(-60 * 24 * time.Hour),
			LastRun: time.Now().Add(-2 * time.Hour), LastStatus: "failed", PassRateLast7d: 80,
		},
		{
			Name: "api-load-test", Namespace: "testkube", Type: "k6", Created: time.Now().Add(-90 * 24 * time.Hour),
			LastRun: time.Now().Add(-5 * time.Hour), LastStatus: "passed", PassRateLast7d: 100,
		},
		{
			Name: "cluster-security", Namespace: "testkube", Type: "trivy", Created: time.Now().Add(-10 * 24 * time.Hour),
			LastRun: time.Now().Add(-24 * time.Hour), LastStatus: "passed", PassRateLast7d: 100,
		},
		{
			Name: "k8s-compliance", Namespace: "testkube", Type: "kubescape", Created: time.Now().Add(-15 * 24 * time.Hour),
			LastRun: time.Now().Add(-48 * time.Hour), LastStatus: "failed", PassRateLast7d: 50,
		},
		{
			Name: "code-quality", Namespace: "testkube", Type: "sonarqube", Created: time.Now().Add(-5 * 24 * time.Hour),
			LastRun: time.Now().Add(-30 * time.Minute), LastStatus: "passed", PassRateLast7d: 90,
		},
		{
			Name: "static-analysis", Namespace: "testkube", Type: "semgrep", Created: time.Now().Add(-2 * 24 * time.Hour),
			LastRun: time.Now().Add(-4 * time.Hour), LastStatus: "passed", PassRateLast7d: 100,
		},
		{
			Name: "vulnerability-management", Namespace: "testkube", Type: "defectdojo", Created: time.Now().Add(-1 * 24 * time.Hour),
			LastRun: time.Now().Add(-12 * time.Hour), LastStatus: "passed", PassRateLast7d: 100,
		},
		{
			Name: "chaos-experiment", Namespace: "testkube", Type: "chaosmesh", Created: time.Now().Add(-20 * 24 * time.Hour),
			LastRun: time.Now().Add(-3 * 24 * time.Hour), LastStatus: "passed", PassRateLast7d: 100,
		},
		{
			Name: "observability-check", Namespace: "testkube", Type: "signoz", Created: time.Now().Add(-3 * 24 * time.Hour),
			LastRun: time.Now().Add(-6 * time.Hour), LastStatus: "passed", PassRateLast7d: 100,
		},
	}

	// Generate executions
	for i := 0; i < 50; i++ {
		status := "passed"
		if i%7 == 0 {
			status = "failed"
		}

		wf := c.workflows[i%len(c.workflows)]

		c.executions = append(c.executions, Execution{
			ID:           fmt.Sprintf("exec-%d", i),
			Name:         fmt.Sprintf("%s-%d", wf.Name, i),
			WorkflowName: wf.Name,
			Status:       status,
			StartTime:    time.Now().Add(time.Duration(-i) * time.Hour),
			EndTime:      time.Now().Add(time.Duration(-i)*time.Hour + 2*time.Minute),
			Duration:     2 * time.Minute,
			Branch:       "main",
		})
	}
}

func (c *MockClient) GetExecutions(opts ListOptions) ([]Execution, error) {
	// Simple filtering
	var result []Execution
	for _, e := range c.executions {
		if opts.Workflow != "" && e.WorkflowName != opts.Workflow {
			continue
		}
		if opts.Status != "" && e.Status != opts.Status {
			continue
		}
		result = append(result, e)
	}

	// Pagination (naive)
	start := (opts.Page - 1) * opts.PageSize
	if start < 0 { start = 0 }
	if start >= len(result) {
		return []Execution{}, nil
	}
	end := start + opts.PageSize
	if end > len(result) {
		end = len(result)
	}

	return result[start:end], nil
}

func (c *MockClient) GetExecution(id string) (*Execution, error) {
	for _, e := range c.executions {
		if e.ID == id {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("execution not found")
}

func (c *MockClient) GetWorkflows() ([]Workflow, error) {
	return c.workflows, nil
}

func (c *MockClient) GetArtifacts(executionID string) ([]Artifact, error) {
	return []Artifact{
		{Name: "playwright-report.zip", Size: 1024 * 1024, Path: "playwright-report.zip"},
		{Name: "results.json", Size: 1024, Path: "results.json"},
	}, nil
}

func (c *MockClient) DownloadArtifact(executionID, path string) ([]byte, error) {
	// Return empty bytes for now, or a minimal valid zip if needed
	return []byte{}, nil
}
