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
		{Name: "frontend-e2e", Namespace: "testkube", Type: "playwright", Created: time.Now().Add(-30 * 24 * time.Hour)},
		{Name: "backend-integration", Namespace: "testkube", Type: "vitest", Created: time.Now().Add(-60 * 24 * time.Hour)},
		{Name: "api-load-test", Namespace: "testkube", Type: "k6", Created: time.Now().Add(-90 * 24 * time.Hour)},
	}

	// Generate executions
	for i := 0; i < 50; i++ {
		status := "passed"
		if i%7 == 0 {
			status = "failed"
		}

		wf := c.workflows[i%3]

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
