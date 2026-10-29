package worker

import (
	"context"
	"encoding/json"
	"log"
	"path/filepath"
	"time"

	"github.com/testkube/dashboard/internal/database"
	"github.com/testkube/dashboard/internal/testkube"
)

type Worker struct {
	api      testkube.Client
	db       database.Database
	interval time.Duration
}

func NewWorker(api testkube.Client, db database.Database) *Worker {
	return &Worker{
		api:      api,
		db:       db,
		interval: 1 * time.Minute,
	}
}

func (w *Worker) Start(ctx context.Context) {
	log.Println("Starting artifact parsing worker...")
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.processExecutions()
	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping worker...")
			return
		case <-ticker.C:
			w.processExecutions()
		}
	}
}

func (w *Worker) processExecutions() {
	// In a real implementation, we would keep track of the last processed execution
	// or have a queue. For now, we'll fetch recent executions and check if we have data for them.
	// This is a naive implementation.

	// Failed runs carry the results that matter most, so take every status and
	// skip the ones that have not finished yet.
	executions, err := w.api.GetExecutions(testkube.ListOptions{
		PageSize: 20,
	})
	if err != nil {
		log.Printf("Worker: failed to fetch executions: %v", err)
		return
	}

	for _, exec := range executions {
		if exec.Status != "passed" && exec.Status != "failed" {
			continue
		}

		// Store execution details
		if err := w.db.InsertExecution(exec); err != nil {
			log.Printf("Worker: failed to insert execution %s: %v", exec.ID, err)
		}

		// Check if we already have metrics for this execution
		// (optimization to avoid re-downloading)
		// k6 rows are checked too: they have no unique key, so re-parsing
		// a k6-only execution would duplicate them every tick.
		metrics, _ := w.db.GetExecutionMetrics(exec.ID)
		k6Metrics, _ := w.db.GetK6Metrics(exec.ID)
		if len(metrics) > 0 || len(k6Metrics) > 0 {
			continue
		}

		// If no metrics, try to parse artifacts
		w.parseArtifacts(exec)
	}
}

func (w *Worker) parseArtifacts(exec testkube.Execution) {
	log.Printf("Worker: processing execution %s (%s)", exec.ID, exec.WorkflowName)

	artifacts, err := w.api.GetArtifacts(exec.ID)
	if err != nil {
		log.Printf("Worker: failed to get artifacts for %s: %v", exec.ID, err)
		return
	}

	for _, artifact := range artifacts {
		// Identify artifact type and parse
		if isPlaywrightJSON(artifact.Name) {
			w.parsePlaywrightJSON(exec.ID, artifact)
		} else if isK6Summary(artifact.Name) {
			w.parseK6Summary(exec.ID, artifact)
		}
	}
}

func isPlaywrightJSON(name string) bool {
	return filepath.Base(name) == "results.json" || filepath.Base(name) == "test-results.json"
}

func isK6Summary(name string) bool {
	return filepath.Base(name) == "summary.json" && filepath.Dir(name) == "k6-results"
}

// PlaywrightResults is the shape of Playwright's JSON reporter output. The
// test title lives on the spec; describe blocks nest as child suites; each
// spec has one test per project, and each test one result per attempt.
type PlaywrightResults struct {
	Suites []PlaywrightSuite `json:"suites"`
}

type PlaywrightSuite struct {
	Suites []PlaywrightSuite `json:"suites"`
	Specs  []struct {
		Title string `json:"title"`
		File  string `json:"file"`
		Tests []struct {
			ProjectName string `json:"projectName"`
			Results     []struct {
				Status   string `json:"status"`
				Duration int    `json:"duration"`
				Error    struct {
					Message string `json:"message"`
				} `json:"error"`
			} `json:"results"`
		} `json:"tests"`
	} `json:"specs"`
}

// playwrightTestCases flattens a report into one row per spec and project,
// taking the final attempt as the outcome and counting the earlier ones as
// retries.
func playwrightTestCases(executionID string, results PlaywrightResults) []database.TestCase {
	var cases []database.TestCase
	var walk func(suites []PlaywrightSuite)
	walk = func(suites []PlaywrightSuite) {
		for _, suite := range suites {
			for _, spec := range suite.Specs {
				for _, test := range spec.Tests {
					if len(test.Results) == 0 {
						continue
					}
					name := spec.Title
					if test.ProjectName != "" {
						name += " [" + test.ProjectName + "]"
					}
					last := test.Results[len(test.Results)-1]
					cases = append(cases, database.TestCase{
						ExecutionID:  executionID,
						TestName:     name,
						FilePath:     spec.File,
						Status:       last.Status,
						DurationMs:   last.Duration,
						ErrorMessage: last.Error.Message,
						RetryCount:   len(test.Results) - 1,
					})
				}
			}
			walk(suite.Suites)
		}
	}
	walk(results.Suites)
	return cases
}

func (w *Worker) parsePlaywrightJSON(executionID string, artifact testkube.Artifact) {
	data, err := w.api.DownloadArtifact(executionID, artifact.Path)
	if err != nil {
		log.Printf("Worker: failed to download %s: %v", artifact.Path, err)
		return
	}

	var results PlaywrightResults
	if err := json.Unmarshal(data, &results); err != nil {
		log.Printf("Worker: failed to parse Playwright JSON: %v", err)
		return
	}

	for _, tc := range playwrightTestCases(executionID, results) {
		if err := w.db.InsertTestCase(tc); err != nil {
			log.Printf("Worker: failed to insert test case: %v", err)
		}
	}
	log.Printf("Worker: processed Playwright results for %s", executionID)
}

type K6Summary struct {
	Metrics map[string]struct {
		Type   string `json:"type"`
		Values struct {
			Min float64 `json:"min"`
			Max float64 `json:"max"`
			Avg float64 `json:"avg"`
			P90 float64 `json:"p(90)"`
			P95 float64 `json:"p(95)"`
			P99 float64 `json:"p(99)"`
		} `json:"values"`
	} `json:"metrics"`
}

func (w *Worker) parseK6Summary(executionID string, artifact testkube.Artifact) {
	data, err := w.api.DownloadArtifact(executionID, artifact.Path)
	if err != nil {
		log.Printf("Worker: failed to download %s: %v", artifact.Path, err)
		return
	}

	var summary K6Summary
	if err := json.Unmarshal(data, &summary); err != nil {
		log.Printf("Worker: failed to parse K6 summary: %v", err)
		return
	}

	for name, metric := range summary.Metrics {
		rec := database.K6MetricRecord{
			ExecutionID: executionID,
			MetricName:  name,
			MetricType:  metric.Type,
			MinValue:    metric.Values.Min,
			MaxValue:    metric.Values.Max,
			AvgValue:    metric.Values.Avg,
			P95Value:    metric.Values.P95,
			P99Value:    metric.Values.P99,
		}
		if err := w.db.InsertK6Metric(rec); err != nil {
			log.Printf("Worker: failed to insert k6 metric: %v", err)
		}
	}
	log.Printf("Worker: processed K6 results for %s", executionID)
}
