package testkube

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

type MockClient struct {
	executions []Execution
	workflows  []Workflow
	logs       map[string][]string
	mu         sync.RWMutex
}

func NewMockClient() *MockClient {
	c := &MockClient{
		logs: make(map[string][]string),
	}
	c.generateMockData()
	return c
}

func (c *MockClient) generateMockData() {
	c.mu.Lock()
	defer c.mu.Unlock()

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
		{
			Name: "trace-analysis", Namespace: "testkube", Type: "testtrace", Created: time.Now().Add(-4 * 24 * time.Hour),
			LastRun: time.Now().Add(-8 * time.Hour), LastStatus: "passed", PassRateLast7d: 98,
		},
		{
			Name: "cost-estimation", Namespace: "testkube", Type: "infracost", Created: time.Now().Add(-2 * 24 * time.Hour),
			LastRun: time.Now().Add(-1 * time.Hour), LastStatus: "passed", PassRateLast7d: 100,
		},
		{
			Name: "firmware-security", Namespace: "testkube", Type: "emba", Created: time.Now().Add(-10 * 24 * time.Hour),
			LastRun: time.Now().Add(-48 * time.Hour), LastStatus: "failed", PassRateLast7d: 60,
		},
		{
			Name: "mqtt-load-test", Namespace: "testkube", Type: "emqtt-bench", Created: time.Now().Add(-5 * 24 * time.Hour),
			LastRun: time.Now().Add(-2 * time.Hour), LastStatus: "passed", PassRateLast7d: 100,
		},
		{
			Name: "iot-platform-test", Namespace: "testkube", Type: "thingboard", Created: time.Now().Add(-20 * 24 * time.Hour),
			LastRun: time.Now().Add(-5 * 24 * time.Hour), LastStatus: "passed", PassRateLast7d: 95,
		},
		{
			Name: "cluster-certification", Namespace: "testkube", Type: "kubekert", Created: time.Now().Add(-15 * 24 * time.Hour),
			LastRun: time.Now().Add(-12 * time.Hour), LastStatus: "passed", PassRateLast7d: 100,
		},
	}

	// Generate executions
	for i := 0; i < 50; i++ {
		status := "passed"
		if i%7 == 0 {
			status = "failed"
		}

		wf := c.workflows[i%len(c.workflows)]
		id := fmt.Sprintf("exec-%d", i)

		c.executions = append(c.executions, Execution{
			ID:           id,
			Name:         fmt.Sprintf("%s-%d", wf.Name, i),
			WorkflowName: wf.Name,
			Status:       status,
			StartTime:    time.Now().Add(time.Duration(-i) * time.Hour),
			EndTime:      time.Now().Add(time.Duration(-i)*time.Hour + 2*time.Minute),
			Duration:     2 * time.Minute,
			Branch:       "main",
		})

		// Pre-fill logs for historical executions
		c.logs[id] = []string{
			"Initializing test runner...",
			"Cloning repository...",
			"Installing dependencies...",
			"Running tests...",
			"Test passed successfully.",
			"Uploading artifacts...",
			"Done.",
		}
	}
}

func (c *MockClient) GetExecutions(opts ListOptions) ([]Execution, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

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

	// Sort by StartTime DESC
	// (Assuming they are already somewhat sorted, but let's be safe if we were really implementing this)
	// For mock, c.executions is roughly sorted by generation order which is time descending?
	// Actually loop generated 0 to 50, with 0 being NOW. So index 0 is newest.
	// We should probably just return them.

	// Pagination (naive)
	start := (opts.Page - 1) * opts.PageSize
	if start < 0 {
		start = 0
	}
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
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, e := range c.executions {
		if e.ID == id {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("execution not found")
}

func (c *MockClient) GetWorkflows() ([]Workflow, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.workflows, nil
}

func (c *MockClient) GetWorkflow(name string) (*Workflow, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, wf := range c.workflows {
		if wf.Name == name {
			return &wf, nil
		}
	}
	return nil, fmt.Errorf("workflow not found: %s", name)
}

func (c *MockClient) RunWorkflow(name string) (*Execution, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find the workflow
	var workflow *Workflow
	for _, wf := range c.workflows {
		if wf.Name == name {
			workflow = &wf
			break
		}
	}
	if workflow == nil {
		return nil, fmt.Errorf("workflow not found: %s", name)
	}

	// Create a new execution
	newID := fmt.Sprintf("exec-%d", len(c.executions)+1000) // avoid collision
	exec := &Execution{
		ID:           newID,
		Name:         fmt.Sprintf("%s-%d", name, len(c.executions)+1),
		WorkflowName: name,
		Status:       "queued",
		StartTime:    time.Now(),
		Branch:       "main",
	}

	// Prepend to executions (so it appears first)
	c.executions = append([]Execution{*exec}, c.executions...)

	// Initialize logs
	c.logs[newID] = []string{"Job queued..."}

	// Start background simulation
	go c.simulateExecution(newID)

	return exec, nil
}

func (c *MockClient) simulateExecution(id string) {
	// Simulate Queued -> Running
	time.Sleep(2 * time.Second)
	c.updateStatus(id, "running")
	c.appendLog(id, "Job started.")
	c.appendLog(id, "Pulling container image...")

	// Simulate Running steps
	steps := []string{
		"Cloning git repository...",
		"Restoring cache...",
		"Installing npm dependencies...",
		"Running tests...",
	}

	for _, step := range steps {
		time.Sleep(2 * time.Second)
		c.appendLog(id, step)
	}

	// Determine pass/fail (randomly, mostly pass)
	finalStatus := "passed"
	if rand.Intn(5) == 0 {
		finalStatus = "failed"
		c.appendLog(id, "Error: Test suite failed.")
		c.appendLog(id, "Details: 2 tests failed, 48 passed.")
	} else {
		c.appendLog(id, "Success: All tests passed.")
	}

	time.Sleep(1 * time.Second)
	c.appendLog(id, "Uploading artifacts...")
	time.Sleep(1 * time.Second)
	c.appendLog(id, "Workflow finished.")

	c.updateStatus(id, finalStatus)
}

func (c *MockClient) updateStatus(id, status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, e := range c.executions {
		if e.ID == id {
			c.executions[i].Status = status
			if status == "passed" || status == "failed" {
				c.executions[i].EndTime = time.Now()
				c.executions[i].Duration = c.executions[i].EndTime.Sub(c.executions[i].StartTime)
			}
			break
		}
	}
}

func (c *MockClient) appendLog(id, line string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	timestamp := time.Now().Format("15:04:05")
	c.logs[id] = append(c.logs[id], fmt.Sprintf("[%s] %s", timestamp, line))
}

func (c *MockClient) GetArtifacts(executionID string) ([]Artifact, error) {
	// Only return artifacts if finished (simple check)
	c.mu.RLock()
	var status string
	for _, e := range c.executions {
		if e.ID == executionID {
			status = e.Status
			break
		}
	}
	c.mu.RUnlock()

	if status != "passed" && status != "failed" {
		return []Artifact{}, nil
	}

	return []Artifact{
		{Name: "playwright-report.zip", Size: 1024 * 1024, Path: "playwright-report.zip"},
		{Name: "results.json", Size: 1024, Path: "results.json"},
		{Name: "screenshot.png", Size: 512 * 1024, Path: "screenshot.png"},
	}, nil
}

func (c *MockClient) DownloadArtifact(executionID, path string) ([]byte, error) {
	if strings.HasSuffix(path, ".json") {
		return []byte(`{"metrics": {"http_req_duration": {"type": "trend", "values": {"min": 50, "max": 200, "avg": 120, "p(95)": 180, "p(99)": 195}}}}`), nil
	}
	if strings.HasSuffix(path, ".html") {
		return []byte(`<html><body><h1>Mock Report</h1><p>This is a simulated report for execution ` + executionID + `</p></body></html>`), nil
	}
	if strings.HasSuffix(path, ".xml") { // JUnit
		return []byte(`<testsuites><testsuite name="mock" tests="1" failures="0"><testcase name="mock_test" time="0.1"/></testsuite></testsuites>`), nil
	}
	return []byte("mock artifact content"), nil
}

func (c *MockClient) GetExecutionLogs(executionID string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if logs, ok := c.logs[executionID]; ok {
		return strings.Join(logs, "\n"), nil
	}
	return "", fmt.Errorf("logs not found")
}
