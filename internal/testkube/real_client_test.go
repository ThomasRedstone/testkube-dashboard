package testkube

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestRealClient_GetExecutions(t *testing.T) {
	// Mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/v1/test-workflow-executions" {
			// Verify query params
			if r.URL.Query().Get("status") != "passed" {
				t.Errorf("expected status=passed, got %s", r.URL.Query().Get("status"))
			}

			response := struct {
				Results []struct {
					ID       string `json:"id"`
					Name     string `json:"name"`
					Workflow struct {
						Name string `json:"name"`
					} `json:"workflow"`
					Result struct {
						Status    string    `json:"status"`
						StartTime time.Time `json:"startTime"`
						EndTime   time.Time `json:"endTime"`
					} `json:"result"`
				} `json:"results"`
			}{
				Results: []struct {
					ID       string `json:"id"`
					Name     string `json:"name"`
					Workflow struct {
						Name string `json:"name"`
					} `json:"workflow"`
					Result struct {
						Status    string    `json:"status"`
						StartTime time.Time `json:"startTime"`
						EndTime   time.Time `json:"endTime"`
					} `json:"result"`
				}{
					{
						ID:   "123",
						Name: "exec-1",
						Workflow: struct {
							Name string `json:"name"`
						}{Name: "workflow-1"},
						Result: struct {
							Status    string    `json:"status"`
							StartTime time.Time `json:"startTime"`
							EndTime   time.Time `json:"endTime"`
						}{
							Status:    "passed",
							StartTime: time.Now().Add(-1 * time.Minute),
							EndTime:   time.Now(),
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// Set env vars
	os.Setenv("TESTKUBE_API_URL", ts.URL)
	os.Setenv("TESTKUBE_NAMESPACE", "test")
	defer os.Unsetenv("TESTKUBE_API_URL")
	defer os.Unsetenv("TESTKUBE_NAMESPACE")

	client, err := NewRealClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	executions, err := client.GetExecutions(ListOptions{Status: "passed"})
	if err != nil {
		t.Fatalf("GetExecutions failed: %v", err)
	}

	if len(executions) != 1 {
		t.Errorf("expected 1 execution, got %d", len(executions))
	}
	if executions[0].ID != "123" {
		t.Errorf("expected ID 123, got %s", executions[0].ID)
	}
}

func TestRealClient_GetWorkflows(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/v1/test-workflows" {
			response := []struct {
				Name      string    `json:"name"`
				Namespace string    `json:"namespace"`
				Created   time.Time `json:"created"`
				Spec      struct {
					Container struct {
						Image string `json:"image"`
					} `json:"container"`
				} `json:"spec"`
			}{
				{
					Name:      "wf-1",
					Namespace: "test",
					Spec: struct {
						Container struct {
							Image string `json:"image"`
						} `json:"container"`
					}{
						Container: struct {
							Image string `json:"image"`
						}{Image: "k6"},
					},
				},
				{
					Name:      "wf-2",
					Namespace: "test",
					Spec: struct {
						Container struct {
							Image string `json:"image"`
						} `json:"container"`
					}{
						Container: struct {
							Image string `json:"image"`
						}{Image: "trivy"},
					},
				},
				{
					Name:      "wf-3",
					Namespace: "test",
					Spec: struct {
						Container struct {
							Image string `json:"image"`
						} `json:"container"`
					}{
						Container: struct {
							Image string `json:"image"`
						}{Image: "kubescape"},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	os.Setenv("TESTKUBE_API_URL", ts.URL)
	defer os.Unsetenv("TESTKUBE_API_URL")

	client, err := NewRealClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	workflows, err := client.GetWorkflows()
	if err != nil {
		t.Fatalf("GetWorkflows failed: %v", err)
	}

	if len(workflows) != 3 {
		t.Errorf("expected 3 workflows, got %d", len(workflows))
	}
	if workflows[0].Type != "k6" {
		t.Errorf("expected type k6, got %s", workflows[0].Type)
	}
	if workflows[1].Type != "trivy" {
		t.Errorf("expected type trivy, got %s", workflows[1].Type)
	}
	if workflows[2].Type != "kubescape" {
		t.Errorf("expected type kubescape, got %s", workflows[2].Type)
	}
}

func TestExtractWorkflowType(t *testing.T) {
	tests := []struct {
		image    string
		expected string
	}{
		{"playwright:v1", "playwright"},
		{"k6-custom:latest", "k6"},
		{"cypress/included:10.0.0", "cypress"},
		{"aquasec/trivy:latest", "trivy"},
		{"kubescape/kubescape:v2", "kubescape"},
		{"sonarqube:latest", "sonarqube"},
		{"returntocorp/semgrep:latest", "semgrep"},
		{"defectdojo/defectdojo-django:latest", "defectdojo"},
		{"chaos-mesh/chaos-mesh:latest", "chaosmesh"},
		{"signoz/signoz:latest", "signoz"},
		{"testtrace:latest", "testtrace"},
		{"infracost/infracost:latest", "infracost"},
		{"emba:latest", "emba"},
		{"emqtt-bench:latest", "emqtt-bench"},
		{"thingsboard/tb-node:latest", "thingboard"},
		{"kubekert:latest", "kubekert"},
		{"unknown:latest", "custom"},
	}

	for _, tt := range tests {
		result := extractWorkflowType(tt.image)
		if result != tt.expected {
			t.Errorf("extractWorkflowType(%s) = %s, expected %s", tt.image, result, tt.expected)
		}
	}
}
