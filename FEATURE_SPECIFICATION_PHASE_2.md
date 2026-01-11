# Testkube Dashboard - Phase 2: API Integration

## Overview

Phase 2 replaces the mock client with a real Testkube API client, enabling the dashboard to display full execution history (301+ executions) instead of just ephemeral CRDs.

**Timeline:** 1-2 days
**Deliverable:** Working dashboard with complete execution history, logs, and artifact lists

---

## Configuration & Service Discovery

### Environment Variables

The dashboard supports flexible configuration via environment variables to work in multiple deployment scenarios:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TESTKUBE_API_URL` | No | `http://testkube-api-server:8088` | Base URL for Testkube API server |
| `TESTKUBE_NAMESPACE` | No | `testkube` | Kubernetes namespace for TestWorkflows |
| `TESTKUBE_API_TOKEN` | No | (empty) | Optional authentication token for API access |
| `DATABASE_URL` | No | (empty) | PostgreSQL connection string (Phase 3+) |
| `USE_MOCK` | No | `false` | Use mock clients for development/testing |
| `LOG_LEVEL` | No | `info` | Logging level: debug, info, warn, error |

### Deployment Scenarios

#### 1. Kubernetes In-Cluster (Production)

**Scenario:** Dashboard deployed in the same cluster as Testkube

**Configuration:**
```yaml
# testkube/dashboard-deployment.yaml
env:
  - name: TESTKUBE_API_URL
    value: "http://testkube-api-server:8088"
  - name: TESTKUBE_NAMESPACE
    value: "testkube"
```

**How it works:**
- Uses Kubernetes DNS service discovery
- Service name `testkube-api-server` resolves to ClusterIP
- No authentication required (internal network)

#### 2. Local Development with Port-Forward

**Scenario:** Developer running dashboard locally, connecting to remote k8s cluster

**Setup:**
```bash
# Terminal 1: Port-forward Testkube API
kubectl port-forward -n testkube svc/testkube-api-server 8088:8088

# Terminal 2: Run dashboard locally
export TESTKUBE_API_URL="http://localhost:8088"
export TESTKUBE_NAMESPACE="testkube"
go run ./cmd/server/main.go
```

**How it works:**
- Port-forward creates local tunnel to k8s service
- Dashboard connects to localhost instead of k8s DNS
- Full access to real data for testing

#### 3. Local Development with Mock Data

**Scenario:** Developer working offline or without k8s access

**Setup:**
```bash
export USE_MOCK="true"
go run ./cmd/server/main.go
```

**How it works:**
- Uses mock client (current Phase 1 implementation)
- No external dependencies
- Good for UI/template development

#### 4. Remote Deployment (Different Cluster)

**Scenario:** Dashboard in one cluster, Testkube in another

**Configuration:**
```yaml
env:
  - name: TESTKUBE_API_URL
    value: "https://testkube.example.com"
  - name: TESTKUBE_API_TOKEN
    valueFrom:
      secretKeyRef:
        name: testkube-credentials
        key: api-token
```

**How it works:**
- Uses full URL with TLS
- Requires authentication token
- Cross-cluster communication

---

## Implementation

### 1. Real Testkube API Client

Create `internal/testkube/real_client.go`:

```go
package testkube

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

type RealClient struct {
	baseURL    string
	httpClient *http.Client
	token      string
	namespace  string
}

// NewRealClient creates a client that connects to the actual Testkube API server
func NewRealClient() (*RealClient, error) {
	// Get API URL from environment, with sensible default for in-cluster deployment
	baseURL := os.Getenv("TESTKUBE_API_URL")
	if baseURL == "" {
		baseURL = "http://testkube-api-server:8088"
	}

	namespace := os.Getenv("TESTKUBE_NAMESPACE")
	if namespace == "" {
		namespace = "testkube"
	}

	client := &RealClient{
		baseURL:   baseURL,
		namespace: namespace,
		token:     os.Getenv("TESTKUBE_API_TOKEN"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Validate connection
	if err := client.healthCheck(); err != nil {
		return nil, fmt.Errorf("testkube API health check failed: %w", err)
	}

	return client, nil
}

func (c *RealClient) healthCheck() error {
	url := fmt.Sprintf("%s/health", c.baseURL)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy (status: %d)", resp.StatusCode)
	}

	return nil
}

func (c *RealClient) GetExecutions(opts ListOptions) ([]Execution, error) {
	// Build query parameters
	params := url.Values{}
	if opts.PageSize > 0 {
		params.Set("pageSize", fmt.Sprintf("%d", opts.PageSize))
	}
	if opts.Page > 0 {
		params.Set("page", fmt.Sprintf("%d", opts.Page))
	}
	if opts.Status != "" {
		params.Set("status", opts.Status)
	}
	if opts.Workflow != "" {
		params.Set("selector", fmt.Sprintf("testworkflow=%s", opts.Workflow))
	}

	// Make API request
	apiURL := fmt.Sprintf("%s/v1/test-workflow-executions?%s", c.baseURL, params.Encode())
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication if token is set
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResponse struct {
		Results []struct {
			ID     string    `json:"id"`
			Name   string    `json:"name"`
			Number int       `json:"number"`
			Workflow struct {
				Name string `json:"name"`
			} `json:"workflow"`
			Result struct {
				Status    string    `json:"status"`
				StartTime time.Time `json:"startTime"`
				EndTime   time.Time `json:"endTime"`
			} `json:"result"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to our model
	executions := make([]Execution, 0, len(apiResponse.Results))
	for _, item := range apiResponse.Results {
		exec := Execution{
			ID:           item.ID,
			Name:         item.Name,
			WorkflowName: item.Workflow.Name,
			Status:       item.Result.Status,
			StartTime:    item.Result.StartTime,
			EndTime:      item.Result.EndTime,
		}

		if !exec.EndTime.IsZero() {
			exec.Duration = exec.EndTime.Sub(exec.StartTime)
		}

		executions = append(executions, exec)
	}

	return executions, nil
}

func (c *RealClient) GetExecution(id string) (*Execution, error) {
	apiURL := fmt.Sprintf("%s/v1/test-workflow-executions/%s", c.baseURL, id)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("execution %s not found", id)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}

	var apiResponse struct {
		ID     string    `json:"id"`
		Name   string    `json:"name"`
		Number int       `json:"number"`
		Workflow struct {
			Name string `json:"name"`
		} `json:"workflow"`
		Result struct {
			Status    string    `json:"status"`
			StartTime time.Time `json:"startTime"`
			EndTime   time.Time `json:"endTime"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	exec := &Execution{
		ID:           apiResponse.ID,
		Name:         apiResponse.Name,
		WorkflowName: apiResponse.Workflow.Name,
		Status:       apiResponse.Result.Status,
		StartTime:    apiResponse.Result.StartTime,
		EndTime:      apiResponse.Result.EndTime,
	}

	if !exec.EndTime.IsZero() {
		exec.Duration = exec.EndTime.Sub(exec.StartTime)
	}

	return exec, nil
}

func (c *RealClient) GetWorkflows() ([]Workflow, error) {
	apiURL := fmt.Sprintf("%s/v1/test-workflows", c.baseURL)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}

	var apiResponse []struct {
		Name      string    `json:"name"`
		Namespace string    `json:"namespace"`
		Created   time.Time `json:"created"`
		Spec      struct {
			Container struct {
				Image string `json:"image"`
			} `json:"container"`
		} `json:"spec"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	workflows := make([]Workflow, 0, len(apiResponse))
	for _, item := range apiResponse {
		wf := Workflow{
			Name:      item.Name,
			Namespace: item.Namespace,
			Created:   item.Created,
			Type:      extractWorkflowType(item.Spec.Container.Image),
		}
		workflows = append(workflows, wf)
	}

	return workflows, nil
}

func (c *RealClient) GetArtifacts(executionID string) ([]Artifact, error) {
	apiURL := fmt.Sprintf("%s/v1/test-workflow-executions/%s/artifacts", c.baseURL, executionID)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}

	var apiResponse []struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	artifacts := make([]Artifact, 0, len(apiResponse))
	for _, item := range apiResponse {
		artifacts = append(artifacts, Artifact{
			Name: item.Name,
			Size: item.Size,
			Path: item.Name,
		})
	}

	return artifacts, nil
}

func (c *RealClient) DownloadArtifact(executionID, path string) ([]byte, error) {
	apiURL := fmt.Sprintf("%s/v1/test-workflow-executions/%s/artifacts/%s",
		c.baseURL, executionID, url.PathEscape(path))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return data, nil
}

// Helper function to extract workflow type from container image
func extractWorkflowType(image string) string {
	switch {
	case containsIgnoreCase(image, "playwright"):
		return "playwright"
	case containsIgnoreCase(image, "vitest"):
		return "vitest"
	case containsIgnoreCase(image, "k6"):
		return "k6"
	case containsIgnoreCase(image, "postman"):
		return "postman"
	case containsIgnoreCase(image, "cypress"):
		return "cypress"
	default:
		return "custom"
	}
}

func containsIgnoreCase(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return contains(s, substr)
}

func toLower(s string) string {
	// Simple ASCII lowercase (good enough for image names)
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			result[i] = s[i] + 32
		} else {
			result[i] = s[i]
		}
	}
	return string(result)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
```

### 2. Update Main Application

Update `cmd/server/main.go`:

```go
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/testkube/dashboard/internal/database"
	"github.com/testkube/dashboard/internal/server"
	"github.com/testkube/dashboard/internal/testkube"
)

func main() {
	// Determine which client to use
	var api testkube.Client
	var err error

	useMock := os.Getenv("USE_MOCK") == "true"

	if useMock {
		log.Println("Using MOCK Testkube API client (USE_MOCK=true)")
		api = testkube.NewMockClient()
	} else {
		log.Println("Using REAL Testkube API client")
		apiURL := os.Getenv("TESTKUBE_API_URL")
		if apiURL == "" {
			apiURL = "http://testkube-api-server:8088"
		}
		log.Printf("Connecting to Testkube API: %s", apiURL)

		api, err = testkube.NewRealClient()
		if err != nil {
			log.Fatalf("Failed to create Testkube API client: %v", err)
		}
		log.Println("✓ Connected to Testkube API")
	}

	// Database still uses mock for Phase 2 (PostgreSQL comes in Phase 3)
	db := database.NewMockDatabase()

	srv := server.NewServer(api, db)

	port := ":8080"
	log.Printf("Starting Testkube Dashboard on %s", port)
	if err := http.ListenAndServe(port, srv.Router()); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
```

### 3. Update Kubernetes Deployment

Update `testkube/dashboard-deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: testkube-dashboard
  namespace: testkube
spec:
  template:
    spec:
      containers:
        - name: dashboard
          image: localhost:32000/testkube-dashboard:latest
          env:
            # Testkube API Configuration
            - name: TESTKUBE_API_URL
              value: "http://testkube-api-server:8088"
            - name: TESTKUBE_NAMESPACE
              value: "testkube"

            # Development/Testing
            - name: USE_MOCK
              value: "false"  # Set to "true" for mock mode

            # Logging
            - name: LOG_LEVEL
              value: "info"

            # Optional: Authentication (if Testkube API requires it)
            # - name: TESTKUBE_API_TOKEN
            #   valueFrom:
            #     secretKeyRef:
            #       name: testkube-api-credentials
            #       key: token
```

---

## Testing & Validation

### 1. Local Development Testing

```bash
# Test with port-forward
kubectl port-forward -n testkube svc/testkube-api-server 8088:8088

# In another terminal
export TESTKUBE_API_URL="http://localhost:8088"
export USE_MOCK="false"
go run ./cmd/server/main.go

# Verify connection
curl http://localhost:8080/
```

**Expected behavior:**
- Dashboard connects to real Testkube API
- Shows 301+ actual executions (not mock data)
- Clicking on execution shows real details

### 2. In-Cluster Testing

```bash
# Build and deploy
docker build -t localhost:32000/testkube-dashboard:latest .
docker push localhost:32000/testkube-dashboard:latest
kubectl rollout restart deployment/testkube-dashboard -n testkube

# Check logs
kubectl logs -n testkube -l app=testkube-dashboard --tail=20

# Expected log output:
# Using REAL Testkube API client
# Connecting to Testkube API: http://testkube-api-server:8088
# ✓ Connected to Testkube API
# Starting Testkube Dashboard on :8080
```

### 3. Verify API Connectivity

```bash
# Port-forward dashboard
kubectl port-forward -n testkube svc/testkube-dashboard 8080:80

# Test endpoints
curl http://localhost:8080/ | grep "Total Executions"
# Should show actual execution count, not "0"

# Check workflows
curl http://localhost:8080/workflows
# Should list real workflows from cluster
```

### 4. Mock Mode Testing

```bash
# Test that mock mode still works (for CI/development)
export USE_MOCK="true"
go run ./cmd/server/main.go

curl http://localhost:8080/
# Should show mock data with 85.5% pass rate
```

---

## API Reference

### Testkube API Endpoints Used

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/health` | GET | Health check during startup |
| `/v1/test-workflows` | GET | List all TestWorkflows |
| `/v1/test-workflow-executions` | GET | List executions (with pagination/filtering) |
| `/v1/test-workflow-executions/{id}` | GET | Get single execution details |
| `/v1/test-workflow-executions/{id}/artifacts` | GET | List artifacts for execution |
| `/v1/test-workflow-executions/{id}/artifacts/{path}` | GET | Download specific artifact |
| `/v1/test-workflow-executions/{id}/logs` | GET | Get execution logs (future) |

### Query Parameters

**GET /v1/test-workflow-executions:**
- `pageSize` - Number of results per page (default: 20)
- `page` - Page number (1-indexed)
- `status` - Filter by status: `passed`, `failed`, `running`, `queued`
- `selector` - Label selector (e.g., `testworkflow=playwright-e2e`)
- `startDate` - ISO 8601 start date filter (future)
- `endDate` - ISO 8601 end date filter (future)

---

## Environment Configuration Examples

### Development `.env` file

```bash
# .env (for local development)
TESTKUBE_API_URL=http://localhost:8088
TESTKUBE_NAMESPACE=testkube
USE_MOCK=false
LOG_LEVEL=debug
```

### Production ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: testkube-dashboard-config
  namespace: testkube
data:
  TESTKUBE_API_URL: "http://testkube-api-server:8088"
  TESTKUBE_NAMESPACE: "testkube"
  LOG_LEVEL: "info"
```

```yaml
# Reference in Deployment
spec:
  template:
    spec:
      containers:
        - name: dashboard
          envFrom:
            - configMapRef:
                name: testkube-dashboard-config
```

---

## Troubleshooting

### Issue: "connection refused" to API server

**Symptoms:**
```
Failed to create Testkube API client: testkube API health check failed:
connection failed: dial tcp: lookup testkube-api-server: no such host
```

**Solutions:**
1. **In-cluster:** Verify service exists:
   ```bash
   kubectl get svc -n testkube testkube-api-server
   ```

2. **Local dev:** Ensure port-forward is running:
   ```bash
   kubectl port-forward -n testkube svc/testkube-api-server 8088:8088
   ```

3. **Check namespace:** Verify `TESTKUBE_NAMESPACE` matches actual namespace

### Issue: "unhealthy (status: 503)"

**Symptoms:**
```
testkube API health check failed: unhealthy (status: 503)
```

**Solutions:**
1. Check Testkube API server status:
   ```bash
   kubectl get pods -n testkube | grep api-server
   kubectl logs -n testkube testkube-api-server-xxx
   ```

2. Verify API server is ready:
   ```bash
   kubectl exec -n testkube testkube-api-server-xxx -- wget -qO- http://localhost:8088/health
   ```

### Issue: Dashboard shows 0 executions despite API connection

**Symptoms:**
- Logs show "✓ Connected to Testkube API"
- Dashboard displays "Total Executions: 0"

**Solutions:**
1. Verify API has executions:
   ```bash
   kubectl exec -n testkube testkube-api-server-xxx -- \
     wget -qO- http://localhost:8088/v1/test-workflow-executions | jq '.results | length'
   ```

2. Check API response format matches code expectations
3. Enable debug logging: `LOG_LEVEL=debug`

### Issue: Local development can't connect

**Symptoms:**
```
TESTKUBE_API_URL=http://localhost:8088 but connection refused
```

**Solutions:**
1. Ensure port-forward is running in separate terminal
2. Check port isn't already in use: `lsof -i :8088`
3. Try explicit namespace: `kubectl port-forward -n testkube svc/testkube-api-server 8088:8088`

---

## Success Criteria

Phase 2 is complete when:

- ✅ Dashboard connects to real Testkube API (not CRDs)
- ✅ Shows all 301+ executions from API server
- ✅ Execution details page displays real metadata
- ✅ Workflows page lists actual TestWorkflows
- ✅ Works in both local dev and in-cluster deployments
- ✅ Environment variables properly configure API URL
- ✅ Health check validates API connectivity on startup
- ✅ Mock mode still works for testing (USE_MOCK=true)
- ✅ Logs clearly indicate which mode is active
- ✅ No errors in production deployment logs

---

## Next Steps (Phase 3)

After Phase 2 is complete:
1. Deploy PostgreSQL for historical metrics
2. Implement background job to parse execution artifacts
3. Store test case details in database for trend analysis
4. Build flaky test detection queries

The real Testkube API client built in Phase 2 will be reused throughout all future phases.
