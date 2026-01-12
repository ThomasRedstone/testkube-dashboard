package testkube

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
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

	// Make API request
	apiURL := fmt.Sprintf("%s/v1/test-workflow-executions?%s", c.baseURL, params.Encode())
	if opts.Workflow != "" {
		apiURL = fmt.Sprintf("%s/v1/test-workflows/%s/executions?%s", c.baseURL, opts.Workflow, params.Encode())
	}
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

		// Enrich with execution data
		executions, err := c.GetExecutions(ListOptions{
			Workflow: item.Name,
			PageSize: 10,
		})
		if err == nil && len(executions) > 0 {
			// Get latest execution for LastRun and LastStatus
			wf.LastRun = executions[0].StartTime
			wf.LastStatus = executions[0].Status

			// Calculate pass rate for last 7 days
			sevenDaysAgo := time.Now().AddDate(0, 0, -7)
			passed := 0
			total := 0
			for _, exec := range executions {
				if exec.StartTime.After(sevenDaysAgo) {
					total++
					if exec.Status == "passed" {
						passed++
					}
				}
			}
			if total > 0 {
				wf.PassRateLast7d = (passed * 100) / total
			}
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

func (c *RealClient) GetWorkflow(name string) (*Workflow, error) {
	apiURL := fmt.Sprintf("%s/v1/test-workflows/%s", c.baseURL, name)
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
		return nil, fmt.Errorf("workflow %s not found", name)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}

	var apiResponse struct {
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

	wf := &Workflow{
		Name:      apiResponse.Name,
		Namespace: apiResponse.Namespace,
		Created:   apiResponse.Created,
		Type:      extractWorkflowType(apiResponse.Spec.Container.Image),
	}

	return wf, nil
}

func (c *RealClient) RunWorkflow(name string) (*Execution, error) {
	apiURL := fmt.Sprintf("%s/v1/test-workflows/%s/executions", c.baseURL, name)
	req, err := http.NewRequest("POST", apiURL, strings.NewReader("{}"))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var apiResponse struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Number int    `json:"number"`
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

	return exec, nil
}

func (c *RealClient) GetExecutionLogs(executionID string) (string, error) {
	apiURL := fmt.Sprintf("%s/v1/test-workflow-executions/%s/logs", c.baseURL, executionID)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(data), nil
}

// Helper function to extract workflow type from container image
func extractWorkflowType(image string) string {
	lowerImage := strings.ToLower(image)
	switch {
	case strings.Contains(lowerImage, "playwright"):
		return "playwright"
	case strings.Contains(lowerImage, "vitest"):
		return "vitest"
	case strings.Contains(lowerImage, "k6"):
		return "k6"
	case strings.Contains(lowerImage, "postman"):
		return "postman"
	case strings.Contains(lowerImage, "cypress"):
		return "cypress"
	case strings.Contains(lowerImage, "trivy"):
		return "trivy"
	case strings.Contains(lowerImage, "kubescape"):
		return "kubescape"
	case strings.Contains(lowerImage, "sonarqube"):
		return "sonarqube"
	case strings.Contains(lowerImage, "semgrep"):
		return "semgrep"
	case strings.Contains(lowerImage, "defectdojo") || strings.Contains(lowerImage, "defect-dojo"):
		return "defectdojo"
	case strings.Contains(lowerImage, "chaos-mesh") || strings.Contains(lowerImage, "chaosmesh"):
		return "chaosmesh"
	case strings.Contains(lowerImage, "signoz"):
		return "signoz"
	case strings.Contains(lowerImage, "testtrace"):
		return "testtrace"
	case strings.Contains(lowerImage, "infracost"):
		return "infracost"
	case strings.Contains(lowerImage, "emba"):
		return "emba"
	case strings.Contains(lowerImage, "emqtt-bench"):
		return "emqtt-bench"
	case strings.Contains(lowerImage, "thingboard") || strings.Contains(lowerImage, "thingsboard"):
		return "thingboard"
	case strings.Contains(lowerImage, "kubekert"):
		return "kubekert"
	default:
		return "custom"
	}
}
