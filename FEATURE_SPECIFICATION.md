# Testkube Dashboard - Feature Specification

## Overview
A lightweight, self-hosted dashboard for Testkube that provides complete test visibility by integrating execution history from the Testkube API and rendering detailed test reports directly from artifacts stored in Testkube's MinIO storage. Built with **htmx + Go**, this eliminates the need for both the $400/month commercial UI and a separate Allure/Java-based report server.

**Key Design Principles:**
- ✅ **htmx-first**: Server-side rendering, minimal JavaScript
- ✅ **Go-native**: All chart generation, parsing, and rendering in Go
- ✅ **Multi-framework**: Playwright, Vitest, k6 - unified interface
- ✅ **Historical intelligence**: Track trends, detect flaky tests, compare runs
- ✅ **Zero vendor lock-in**: Open formats, self-hosted, extensible

---

## Current vs. Proposed Architecture

### Current State (Suboptimal)
```
┌─────────────┐
│ Test Runner │
└──────┬──────┘
       │ generates reports
       ├─────────────────────┬────────────────────┐
       │                     │                    │
       ▼                     ▼                    ▼
┌─────────────┐     ┌──────────────┐    ┌─────────────┐
│ Testkube    │     │ Allure Server│    │ Testkube    │
│ API         │     │ (Java, curl) │    │ MinIO       │
│ (metadata)  │     │ (HTML only)  │    │ (metrics)   │
└─────────────┘     └──────────────┘    └─────────────┘
```

**Problems:**
- Playwright/Vitest/k6 reports uploaded to separate Allure Server via curl
- Allure requires Java runtime (150MB+ image bloat)
- Currently Playwright/Vitest HTML artifacts NOT saved to Testkube MinIO
- No historical trend tracking
- No cross-run analysis (flaky tests, regressions)
- Three separate systems to manage

### Proposed Architecture (Unified)
```
┌─────────────────────────────────────────────────┐
│ Test Runners                                    │
│ ┌──────────┐  ┌────────┐  ┌──────┐            │
│ │Playwright│  │ Vitest │  │  k6  │            │
│ └────┬─────┘  └───┬────┘  └───┬──┘            │
│      │            │            │                │
│      ▼            ▼            ▼                │
│   HTML         HTML         JSON               │
└──────┬───────────┬────────────┬────────────────┘
       │           │            │
       ▼           ▼            ▼
┌──────────────────────────────────────┐
│ Testkube MinIO (Artifact Storage)    │
│ ┌──────────────────────────────────┐ │
│ │ playwright-report/index.html     │ │
│ │ test-results/index.html (Vitest) │ │
│ │ k6/summary.json                  │ │
│ │ videos/screenshots/traces        │ │
│ └──────────────────────────────────┘ │
└───────────────┬──────────────────────┘
                │
                ├─────────────────┬──────────────────┐
                ▼                 ▼                  ▼
         ┌────────────┐    ┌────────────┐   ┌────────────┐
         │ Testkube   │    │ Dashboard  │   │ PostgreSQL │
         │ API        │    │ (Go/htmx)  │   │ (Trends)   │
         │            │    │            │   │            │
         │            │───▶│ Parses     │──▶│ Metrics    │
         │            │    │ Artifacts  │   │ History    │
         └────────────┘    └────────────┘   └────────────┘
                                 │
                                 ▼
                          ┌──────────────┐
                          │   Browser    │
                          │  (htmx +     │
                          │   Alpine.js) │
                          └──────────────┘
```

**Benefits:**
- ✅ Single source of truth (everything in Testkube MinIO)
- ✅ Unified retention policy
- ✅ No Java dependency
- ✅ Historical trend tracking built-in
- ✅ Flaky test detection across runs
- ✅ Performance regression alerts
- ✅ Simpler architecture, lower operational overhead

---

## Core Functionality

### 1. Dashboard Overview Page
**What it does:**
- Displays aggregate metrics across all test workflows
- Shows recent test activity and historical trends
- Highlights current failures and flaky tests requiring attention
- Cross-framework metrics (Playwright + Vitest + k6 unified)

**Implementation:**
```go
// Query historical database + latest executions
func (s *Server) GetDashboardSummary(ctx context.Context) (*DashboardData, error) {
    // Recent executions from Testkube API
    executions := s.testkubeAPI.GetExecutions(pageSize: 50)

    // Historical metrics from PostgreSQL
    trends := s.db.GetTrends(last: 30*24*time.Hour)

    // Flaky test detection
    flakyTests := s.db.GetFlakyTests(threshold: 0.3) // 30% inconsistency

    return &DashboardData{
        TotalTests: len(workflows),
        PassRate: trends.CurrentPassRate,
        PassRateTrend: trends.PassRateChange, // "+5.2% this week"
        AvgDuration: trends.AvgDuration,
        DurationTrend: trends.DurationChange, // "-12% faster"
        RecentFailures: executions.Failed,
        FlakyTests: flakyTests,
        ActiveRuns: executions.Running,
        Charts: []Chart{
            s.generatePassRateChart(trends),
            s.generateDurationChart(trends),
            s.generateTestVolumeChart(trends),
        },
    }
}
```

**Display (htmx template):**
```html
<div class="dashboard-grid">
    <div class="metric-card">
        <h3>Pass Rate</h3>
        <div class="stat">{{.PassRate}}%</div>
        <div class="trend {{if gt .PassRateTrend 0}}up{{else}}down{{end}}">
            {{.PassRateTrend}}
        </div>
        <div class="chart">
            {{template "chart-svg" .PassRateChart}}
        </div>
    </div>
    <!-- More cards... -->
</div>

<div class="flaky-tests-alert" hx-get="/api/flaky-tests" hx-trigger="every 5m">
    <!-- Auto-updates via htmx -->
</div>
```

**Charts (Server-side Go generation):**
- Use `go-echarts` or `go-chart` library
- Generate SVG charts server-side
- Embed in HTML templates
- No JavaScript required for rendering
- Optional: Add htmx for interactive drill-down

---

### 2. Test Workflow List
**What it does:**
- Lists all available test workflows with their latest execution status
- Shows historical pass rate per workflow
- Quick access to run tests or view detailed history

**Implementation:**
```go
type WorkflowSummary struct {
    Name           string
    Type           string  // "playwright", "vitest", "k6"
    LastRun        time.Time
    LastStatus     string
    LastDuration   time.Duration
    PassRateLast7d float64
    TotalRuns      int
    FlakyTests     int
}

// Fetch from Testkube API + historical DB
workflows := s.getWorkflowsWithMetrics()
```

**Display:**
```html
<table class="workflows-table">
    <thead>
        <tr>
            <th>Workflow</th>
            <th>Type</th>
            <th>Last Run</th>
            <th>Status</th>
            <th>Pass Rate (7d)</th>
            <th>Actions</th>
        </tr>
    </thead>
    <tbody>
    {{range .Workflows}}
        <tr>
            <td><a href="/workflows/{{.Name}}">{{.Name}}</a></td>
            <td><span class="badge badge-{{.Type}}">{{.Type}}</span></td>
            <td>{{.LastRun.Format "2006-01-02 15:04"}}</td>
            <td><span class="status status-{{.LastStatus}}">{{.LastStatus}}</span></td>
            <td>
                {{.PassRateLast7d}}%
                {{template "sparkline-svg" .PassRateHistory}}
            </td>
            <td>
                <button class="btn" hx-post="/workflows/{{.Name}}/run" hx-swap="none">
                    Run
                </button>
                <a href="/workflows/{{.Name}}/history" class="btn-link">History</a>
            </td>
        </tr>
    {{end}}
    </tbody>
</table>
```

---

### 3. Test Execution History
**What it does:**
- Shows complete execution history for a specific test workflow
- Filterable by status, date range, branch
- Direct access to detailed test reports rendered in-dashboard
- Shows trends over time

**Implementation:**
```go
func (s *Server) GetExecutionHistory(workflow string, filters Filters) (*HistoryView, error) {
    // Fetch from Testkube API with pagination
    executions := s.testkubeAPI.GetExecutions(
        workflow: workflow,
        pageSize: 50,
        filters: filters,
    )

    // Get historical metrics for comparison
    metrics := s.db.GetWorkflowMetrics(workflow, last: 90*24*time.Hour)

    return &HistoryView{
        Executions: executions,
        Trends: s.generateTrendCharts(metrics),
        Filters: filters,
    }
}
```

**Display:**
```html
<!-- Filter bar -->
<form hx-get="/workflows/{{.Name}}/history" hx-target="#execution-list">
    <select name="status">
        <option value="">All Statuses</option>
        <option value="passed">Passed</option>
        <option value="failed">Failed</option>
    </select>
    <input type="date" name="start_date" />
    <input type="date" name="end_date" />
    <input type="text" name="branch" placeholder="Branch..." />
    <button type="submit">Filter</button>
</form>

<!-- Trend chart -->
<div class="trend-chart">
    {{template "pass-rate-trend-svg" .Trends}}
</div>

<!-- Execution table -->
<table id="execution-list">
    <thead>
        <tr>
            <th>Execution #</th>
            <th>Status</th>
            <th>Started</th>
            <th>Duration</th>
            <th>Branch</th>
            <th>Report</th>
        </tr>
    </thead>
    <tbody>
    {{range .Executions}}
        <tr>
            <td><a href="/executions/{{.ID}}">{{.Number}}</a></td>
            <td><span class="status status-{{.Status}}">{{.Status}}</span></td>
            <td>{{.StartTime.Format "2006-01-02 15:04"}}</td>
            <td>{{.Duration}}</td>
            <td>{{.Branch}}</td>
            <td>
                <a href="/executions/{{.ID}}/report" class="btn-primary">
                    View Report
                </a>
            </td>
        </tr>
    {{end}}
    </tbody>
</table>

<!-- Pagination via htmx -->
<div hx-get="/workflows/{{.Name}}/history?page={{.NextPage}}"
     hx-trigger="intersect once"
     hx-swap="afterend">
    Loading more...
</div>
```

---

### 4. Execution Details & Report Viewer

#### 4a. Execution Metadata & Logs
**What it does:**
- Shows detailed information about a single test execution
- Real-time logs for running tests
- Quick access to all artifacts
- Per-test breakdown with drill-down

**Implementation:**
```go
func (s *Server) GetExecutionDetails(id string) (*ExecutionDetail, error) {
    // Fetch from Testkube API
    execution := s.testkubeAPI.GetExecution(id)

    // Get parsed metrics from database
    metrics := s.db.GetExecutionMetrics(id)

    // Get artifact list
    artifacts := s.testkubeAPI.GetArtifacts(id)

    return &ExecutionDetail{
        Execution: execution,
        Metrics: metrics,
        Artifacts: artifacts,
        TestCases: metrics.TestCases, // Parsed from artifacts
    }
}
```

**Display:**
```html
<div class="execution-header">
    <h1>Execution #{{.Execution.Number}}</h1>
    <span class="status-badge status-{{.Execution.Status}}">{{.Execution.Status}}</span>
</div>

<div class="execution-metadata">
    <div class="meta-item">
        <label>Workflow:</label>
        <span>{{.Execution.WorkflowName}}</span>
    </div>
    <div class="meta-item">
        <label>Duration:</label>
        <span>{{.Execution.Duration}}</span>
    </div>
    <div class="meta-item">
        <label>Branch:</label>
        <span>{{.Execution.Branch}}</span>
    </div>
</div>

<!-- Test breakdown -->
<div class="test-breakdown">
    <h2>Test Cases ({{len .TestCases}})</h2>
    <table>
        <thead>
            <tr>
                <th>Test Name</th>
                <th>Status</th>
                <th>Duration</th>
                <th>History</th>
            </tr>
        </thead>
        <tbody>
        {{range .TestCases}}
            <tr class="test-row test-{{.Status}}">
                <td>{{.Name}}</td>
                <td><span class="status-{{.Status}}">{{.Status}}</span></td>
                <td>{{.Duration}}</td>
                <td>
                    <!-- htmx loads sparkline on hover -->
                    <div hx-get="/tests/{{.Name}}/sparkline"
                         hx-trigger="mouseenter once"
                         hx-swap="innerHTML">
                        ...
                    </div>
                </td>
            </tr>
        {{end}}
        </tbody>
    </table>
</div>

<!-- Console logs (live-updating via htmx SSE) -->
<div class="logs-section">
    <h2>Console Logs</h2>
    <pre hx-get="/executions/{{.Execution.ID}}/logs"
         hx-trigger="every 2s"
         hx-swap="innerHTML">{{.Logs}}</pre>
</div>

<!-- Primary action -->
<div class="report-actions">
    <a href="/executions/{{.Execution.ID}}/report" class="btn-primary">
        View Full Test Report
    </a>
</div>
```

#### 4b. Multi-Framework Report Viewer

**Playwright Reports (HTML - Serve Directly):**
```go
func (s *Server) handlePlaywrightReport(w http.ResponseWriter, r *http.Request) {
    executionID := chi.URLParam(r, "id")

    // Check cache
    if cached := s.cache.Get(executionID); cached != nil {
        http.ServeFile(w, r, cached.Path)
        return
    }

    // Download playwright-report/** from Testkube API
    reportDir := s.downloadArtifacts(executionID, "playwright-report/**/*")

    // Cache for 10 minutes
    s.cache.Set(executionID, reportDir, 10*time.Minute)

    // Serve index.html
    http.ServeFile(w, r, filepath.Join(reportDir, "index.html"))
}

// Also handle sub-resources (CSS, JS, videos, traces)
func (s *Server) handlePlaywrightAsset(w http.ResponseWriter, r *http.Request) {
    executionID := chi.URLParam(r, "id")
    assetPath := chi.URLParam(r, "*")

    reportDir := s.cache.Get(executionID)
    http.ServeFile(w, r, filepath.Join(reportDir, assetPath))
}
```

**Vitest Reports (HTML - Serve Directly):**
```go
func (s *Server) handleVitestReport(w http.ResponseWriter, r *http.Request) {
    executionID := chi.URLParam(r, "id")

    // Similar to Playwright - download test-results/index.html and serve
    reportDir := s.downloadArtifacts(executionID, "test-results/**/*")
    http.ServeFile(w, r, filepath.Join(reportDir, "index.html"))
}
```

**k6 Reports (Generate with Go):**
```go
func (s *Server) handleK6Report(w http.ResponseWriter, r *http.Request) {
    executionID := chi.URLParam(r, "id")

    // Download k6 summary.json
    summaryPath := s.downloadArtifact(executionID, "k6/summary.json")

    // Parse JSON
    var summary K6Summary
    json.Unmarshal(readFile(summaryPath), &summary)

    // Generate charts server-side with go-echarts
    charts := s.generateK6Charts(summary)

    // Render template
    s.render(w, "k6_report.html", struct {
        Summary K6Summary
        Charts  []Chart
    }{summary, charts})
}

type K6Summary struct {
    Metrics map[string]K6Metric `json:"metrics"`
}

type K6Metric struct {
    Type   string `json:"type"`
    Values struct {
        Min float64 `json:"min"`
        Max float64 `json:"max"`
        Avg float64 `json:"avg"`
        P95 float64 `json:"p(95)"`
        P99 float64 `json:"p(99)"`
    } `json:"values"`
}

func (s *Server) generateK6Charts(summary K6Summary) []Chart {
    charts := []Chart{}

    // Response time distribution chart (go-echarts)
    httpReqDuration := summary.Metrics["http_req_duration"]
    charts = append(charts, s.generateBarChart(
        "Response Time Distribution",
        []string{"Min", "Avg", "P95", "P99", "Max"},
        []float64{
            httpReqDuration.Values.Min,
            httpReqDuration.Values.Avg,
            httpReqDuration.Values.P95,
            httpReqDuration.Values.P99,
            httpReqDuration.Values.Max,
        },
    ))

    // Request rate over time (if detailed results.json available)
    // ...

    return charts
}
```

**k6 Report Template (Go template):**
```html
{{define "k6_report.html"}}
<div class="k6-report">
    <h1>k6 Load Test Report</h1>

    <div class="metrics-grid">
        {{range $name, $metric := .Summary.Metrics}}
        <div class="metric-card">
            <h3>{{$name}}</h3>
            <table>
                <tr><td>Min:</td><td>{{$metric.Values.Min}}</td></tr>
                <tr><td>Avg:</td><td>{{$metric.Values.Avg}}</td></tr>
                <tr><td>P95:</td><td>{{$metric.Values.P95}}</td></tr>
                <tr><td>P99:</td><td>{{$metric.Values.P99}}</td></tr>
                <tr><td>Max:</td><td>{{$metric.Values.Max}}</td></tr>
            </table>
        </div>
        {{end}}
    </div>

    <div class="charts-section">
        {{range .Charts}}
            {{template "chart-svg" .}}
        {{end}}
    </div>
</div>
{{end}}
```

---

### 5. Historical Trends & Analytics

**What it does:**
- Track test metrics over time (all frameworks)
- Detect flaky tests automatically
- Identify performance regressions
- Compare current run vs. historical baseline

**Database Schema:**
```sql
-- Main execution tracking
CREATE TABLE test_executions (
    id TEXT PRIMARY KEY,
    workflow_name TEXT NOT NULL,
    workflow_type TEXT, -- 'playwright', 'vitest', 'k6'
    status TEXT NOT NULL,
    started_at TIMESTAMP NOT NULL,
    finished_at TIMESTAMP,
    duration_ms INTEGER,
    branch TEXT,
    commit_sha TEXT,
    config JSONB
);

-- Per-test case tracking (Playwright/Vitest)
CREATE TABLE test_cases (
    id SERIAL PRIMARY KEY,
    execution_id TEXT REFERENCES test_executions(id) ON DELETE CASCADE,
    test_name TEXT NOT NULL,
    file_path TEXT,
    status TEXT NOT NULL, -- 'passed', 'failed', 'skipped', 'flaky'
    duration_ms INTEGER,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(execution_id, test_name)
);

-- k6 metrics tracking
CREATE TABLE k6_metrics (
    id SERIAL PRIMARY KEY,
    execution_id TEXT REFERENCES test_executions(id) ON DELETE CASCADE,
    metric_name TEXT NOT NULL,
    metric_type TEXT, -- 'counter', 'gauge', 'trend', 'rate'
    min_value FLOAT,
    max_value FLOAT,
    avg_value FLOAT,
    p95_value FLOAT,
    p99_value FLOAT,
    count INTEGER,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Flaky test tracking
CREATE TABLE flaky_tests (
    test_name TEXT PRIMARY KEY,
    total_runs INTEGER DEFAULT 0,
    failed_runs INTEGER DEFAULT 0,
    passed_runs INTEGER DEFAULT 0,
    flaky_score FLOAT, -- 0.0 to 1.0, higher = more flaky
    first_seen TIMESTAMP,
    last_seen TIMESTAMP,
    last_failure TIMESTAMP
);

-- Indexes for performance
CREATE INDEX idx_test_cases_name ON test_cases(test_name);
CREATE INDEX idx_test_cases_status ON test_cases(status, created_at);
CREATE INDEX idx_executions_workflow ON test_executions(workflow_name, started_at DESC);
CREATE INDEX idx_k6_metrics_name ON k6_metrics(metric_name, execution_id);
CREATE INDEX idx_flaky_tests_score ON flaky_tests(flaky_score DESC);
```

**Background Job - Parse Artifacts:**
```go
// Triggered when execution completes
func (s *Server) ParseExecutionArtifacts(executionID string) error {
    execution := s.testkubeAPI.GetExecution(executionID)

    // Store main execution record
    s.db.InsertExecution(execution)

    // Download and parse based on workflow type
    switch execution.WorkflowType() {
    case "playwright":
        s.parsePlaywrightResults(executionID)
    case "vitest":
        s.parseVitestResults(executionID)
    case "k6":
        s.parseK6Results(executionID)
    }

    // Update flaky test detection
    s.updateFlakyTestScores()

    return nil
}

func (s *Server) parsePlaywrightResults(executionID string) error {
    // Download test-results/*.json (Playwright JSON reporter output)
    resultsPath := s.downloadArtifact(executionID, "test-results/results.json")

    var results PlaywrightResults
    json.Unmarshal(readFile(resultsPath), &results)

    for _, suite := range results.Suites {
        for _, spec := range suite.Specs {
            for _, test := range spec.Tests {
                s.db.InsertTestCase(TestCase{
                    ExecutionID:  executionID,
                    TestName:     test.Title,
                    FilePath:     spec.File,
                    Status:       test.Status,
                    DurationMs:   test.Duration,
                    ErrorMessage: test.Error,
                    RetryCount:   test.Retries,
                })
            }
        }
    }

    return nil
}

func (s *Server) parseVitestResults(executionID string) error {
    // Parse JUnit XML or Vitest JSON output
    junitPath := s.downloadArtifact(executionID, "test-results/junit.xml")

    results := parseJUnitXML(junitPath)

    for _, testCase := range results.TestCases {
        s.db.InsertTestCase(TestCase{
            ExecutionID: executionID,
            TestName:    testCase.Name,
            FilePath:    testCase.ClassName,
            Status:      testCase.Status,
            DurationMs:  int(testCase.Duration * 1000),
            ErrorMessage: testCase.Failure,
        })
    }

    return nil
}

func (s *Server) parseK6Results(executionID string) error {
    summaryPath := s.downloadArtifact(executionID, "k6/summary.json")

    var summary K6Summary
    json.Unmarshal(readFile(summaryPath), &summary)

    for name, metric := range summary.Metrics {
        s.db.InsertK6Metric(K6MetricRecord{
            ExecutionID: executionID,
            MetricName:  name,
            MetricType:  metric.Type,
            MinValue:    metric.Values.Min,
            MaxValue:    metric.Values.Max,
            AvgValue:    metric.Values.Avg,
            P95Value:    metric.Values.P95,
            P99Value:    metric.Values.P99,
        })
    }

    return nil
}
```

**Flaky Test Detection:**
```go
func (s *Server) updateFlakyTestScores() error {
    // For each test, calculate flaky score based on recent history
    query := `
        WITH test_stats AS (
            SELECT
                test_name,
                COUNT(*) as total_runs,
                SUM(CASE WHEN status = 'passed' THEN 1 ELSE 0 END) as passed,
                SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed,
                MAX(created_at) as last_seen,
                MAX(CASE WHEN status = 'failed' THEN created_at END) as last_failure
            FROM test_cases
            WHERE created_at > NOW() - INTERVAL '30 days'
            GROUP BY test_name
            HAVING COUNT(*) >= 5  -- Minimum runs to detect pattern
        )
        INSERT INTO flaky_tests (test_name, total_runs, failed_runs, passed_runs, flaky_score, last_seen, last_failure)
        SELECT
            test_name,
            total_runs,
            failed,
            passed,
            -- Flaky score: tests that intermittently fail (not always pass or always fail)
            CASE
                WHEN passed > 0 AND failed > 0 THEN
                    -- Higher score for tests that fail 20-80% of the time
                    1.0 - ABS(0.5 - (failed::float / total_runs)) * 2
                ELSE 0.0
            END as flaky_score,
            last_seen,
            last_failure
        FROM test_stats
        ON CONFLICT (test_name)
        DO UPDATE SET
            total_runs = EXCLUDED.total_runs,
            failed_runs = EXCLUDED.failed_runs,
            passed_runs = EXCLUDED.passed_runs,
            flaky_score = EXCLUDED.flaky_score,
            last_seen = EXCLUDED.last_seen,
            last_failure = EXCLUDED.last_failure;
    `

    s.db.Exec(query)
    return nil
}
```

**Trend Queries:**
```go
func (s *Server) GetPassRateTrend(workflow string, days int) ([]DataPoint, error) {
    query := `
        SELECT
            DATE(started_at) as day,
            COUNT(*) as total,
            SUM(CASE WHEN status = 'passed' THEN 1 ELSE 0 END) as passed,
            (SUM(CASE WHEN status = 'passed' THEN 1 ELSE 0 END)::float / COUNT(*) * 100) as pass_rate
        FROM test_executions
        WHERE workflow_name = $1
          AND started_at > NOW() - INTERVAL '$2 days'
        GROUP BY DATE(started_at)
        ORDER BY day ASC
    `

    rows := s.db.Query(query, workflow, days)
    // Parse and return data points
}

func (s *Server) GetDurationTrend(workflow string, days int) ([]DataPoint, error) {
    query := `
        SELECT
            DATE(started_at) as day,
            AVG(duration_ms) as avg_duration,
            PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms) as p95_duration
        FROM test_executions
        WHERE workflow_name = $1
          AND started_at > NOW() - INTERVAL '$2 days'
          AND status IN ('passed', 'failed') -- exclude cancelled/timeout
        GROUP BY DATE(started_at)
        ORDER BY day ASC
    `

    rows := s.db.Query(query, workflow, days)
    // Parse and return data points
}

func (s *Server) GetFlakyTests(threshold float64) ([]FlakyTest, error) {
    query := `
        SELECT test_name, total_runs, failed_runs, passed_runs, flaky_score, last_failure
        FROM flaky_tests
        WHERE flaky_score >= $1
        ORDER BY flaky_score DESC, last_failure DESC
        LIMIT 20
    `

    rows := s.db.Query(query, threshold)
    // Parse and return flaky tests
}
```

**Chart Generation (Server-side with go-echarts):**
```go
import (
    "github.com/go-echarts/go-echarts/v2/charts"
    "github.com/go-echarts/go-echarts/v2/opts"
)

func (s *Server) generatePassRateChart(data []DataPoint) string {
    line := charts.NewLine()
    line.SetGlobalOptions(
        charts.WithTitleOpts(opts.Title{Title: "Pass Rate Trend"}),
        charts.WithTooltipOpts(opts.Tooltip{Show: true}),
    )

    xAxis := make([]string, len(data))
    yAxis := make([]opts.LineData, len(data))

    for i, dp := range data {
        xAxis[i] = dp.Date.Format("Jan 02")
        yAxis[i] = opts.LineData{Value: dp.PassRate}
    }

    line.SetXAxis(xAxis).
         AddSeries("Pass Rate %", yAxis).
         SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{Smooth: true}))

    // Render to SVG string (or HTML snippet)
    var buf bytes.Buffer
    line.Render(&buf)
    return buf.String()
}

func (s *Server) generateDurationChart(data []DataPoint) string {
    bar := charts.NewBar()
    bar.SetGlobalOptions(
        charts.WithTitleOpts(opts.Title{Title: "Test Duration Trend"}),
    )

    xAxis := make([]string, len(data))
    avgData := make([]opts.BarData, len(data))
    p95Data := make([]opts.BarData, len(data))

    for i, dp := range data {
        xAxis[i] = dp.Date.Format("Jan 02")
        avgData[i] = opts.BarData{Value: dp.AvgDuration}
        p95Data[i] = opts.BarData{Value: dp.P95Duration}
    }

    bar.SetXAxis(xAxis).
        AddSeries("Average", avgData).
        AddSeries("P95", p95Data)

    var buf bytes.Buffer
    bar.Render(&buf)
    return buf.String()
}

func (s *Server) generateK6ResponseTimeChart(metrics K6Metric) string {
    bar := charts.NewBar()
    bar.SetGlobalOptions(
        charts.WithTitleOpts(opts.Title{Title: "Response Time Distribution"}),
        charts.WithYAxisOpts(opts.YAxis{Name: "Milliseconds"}),
    )

    bar.SetXAxis([]string{"Min", "Avg", "P95", "P99", "Max"}).
        AddSeries("http_req_duration", []opts.BarData{
            {Value: metrics.Values.Min},
            {Value: metrics.Values.Avg},
            {Value: metrics.Values.P95},
            {Value: metrics.Values.P99},
            {Value: metrics.Values.Max},
        })

    var buf bytes.Buffer
    bar.Render(&buf)
    return buf.String()
}
```

---

### 6. Search & Filtering

**What it does:**
- Search across all executions by workflow name, status, date, branch
- Save common filters as "views"
- Search within test cases across all executions

**Implementation (htmx-powered):**
```html
<!-- Search bar -->
<form hx-get="/search" hx-target="#search-results" hx-trigger="keyup changed delay:300ms from:input[name=q]">
    <input type="text" name="q" placeholder="Search tests, executions, or workflows..." />
    <select name="type">
        <option value="all">All</option>
        <option value="executions">Executions</option>
        <option value="tests">Test Cases</option>
        <option value="workflows">Workflows</option>
    </select>
    <button type="submit">Search</button>
</form>

<div id="search-results">
    <!-- Results loaded via htmx -->
</div>
```

**Backend:**
```go
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
    query := r.URL.Query().Get("q")
    searchType := r.URL.Query().Get("type")

    var results SearchResults

    switch searchType {
    case "executions":
        results.Executions = s.db.SearchExecutions(query)
    case "tests":
        results.TestCases = s.db.SearchTestCases(query)
    case "workflows":
        results.Workflows = s.db.SearchWorkflows(query)
    default:
        results.Executions = s.db.SearchExecutions(query)
        results.TestCases = s.db.SearchTestCases(query)
        results.Workflows = s.db.SearchWorkflows(query)
    }

    s.render(w, "search_results.html", results)
}

func (db *Database) SearchTestCases(query string) []TestCase {
    sql := `
        SELECT DISTINCT ON (test_name)
            test_name, file_path, status, execution_id, created_at
        FROM test_cases
        WHERE test_name ILIKE $1 OR file_path ILIKE $1
        ORDER BY test_name, created_at DESC
        LIMIT 50
    `

    // Search with wildcards
    rows := db.Query(sql, "%"+query+"%")
    // Parse results...
}
```

---

## Test Framework Integration

### Playwright E2E Tests

**Current Configuration:**
```typescript
// playwright.config.ts
reporters: [
  ['html', { open: 'never' }],
  ['allure-playwright', { resultsDir: 'allure-results' }],  // REMOVE
  ['json', { outputFile: 'test-results/results.json' }],     // ADD
]
```

**Updated TestWorkflow (testkube/e2e-tests.yaml):**
```yaml
steps:
  - name: Run E2E Tests
    shell: |
      npx playwright test --workers=8
      # Generates:
      # - playwright-report/index.html (HTML report)
      # - test-results/results.json (JSON for parsing)
      # - videos/**/*.webm
      # - screenshots/**/*.png
      # - trace/**/*.zip

    artifacts:
      paths:
        - playwright-report/**/*
        - test-results/**/*
        - videos/**/*
        - screenshots/**/*
```

**What Dashboard Does:**
1. **Serve HTML report directly** - `playwright-report/index.html` (includes trace viewer!)
2. **Parse JSON** - Extract test cases from `test-results/results.json`
3. **Store metrics** - Insert into database for historical tracking
4. **Generate trends** - Server-side charts showing pass rate, duration over time

---

### Vitest Unit Tests

**Current Configuration:**
```typescript
// vitest.config.ts
test: {
  reporters: ['verbose', 'junit'],
  coverage: {
    reporter: ['json-summary', 'text', 'html'],
  }
}
```

**Updated Configuration:**
```typescript
// vitest.config.ts
test: {
  reporters: [
    'default',
    'junit',
    'html',  // ADD: Generates test-results/html/index.html
    'json',  // ADD: Generates test-results/.json
  ],
  outputFile: {
    junit: './test-results/junit.xml',
    json: './test-results/results.json',
    html: './test-results/html/index.html',
  },
  coverage: {
    reporter: ['json-summary', 'text', 'html'],
    reportsDirectory: './coverage',
  }
}
```

**Updated TestWorkflow (testkube/unit-tests.yaml):**
```yaml
steps:
  - name: Run Unit Tests
    shell: |
      npm run test:vitest -- \
        --reporter=junit \
        --reporter=html \
        --reporter=json \
        --outputFile.junit=test-results/junit.xml \
        --outputFile.json=test-results/results.json \
        --outputFile.html=test-results/html/index.html \
        --coverage

      # Generates:
      # - test-results/junit.xml (for parsing)
      # - test-results/results.json (detailed JSON)
      # - test-results/html/index.html (HTML report)
      # - coverage/index.html (coverage report)

    artifacts:
      paths:
        - test-results/**/*
        - coverage/**/*
```

**What Dashboard Does:**
1. **Serve HTML report** - `test-results/html/index.html` (Vitest HTML reporter)
2. **Serve coverage report** - `coverage/index.html` (bonus feature)
3. **Parse JUnit XML** - Extract test results from `junit.xml`
4. **Store metrics** - Insert into database
5. **Show coverage trends** - Parse `coverage/coverage-summary.json` over time

---

### k6 Load Tests

**Create New TestWorkflow (testkube/k6-tests.yaml):**
```yaml
apiVersion: testworkflows.testkube.io/v1
kind: TestWorkflow
metadata:
  name: k6-load-tests
  namespace: testkube
  labels:
    app: texecom-cloud
    test-type: performance
spec:
  content:
    git:
      uri: git@bitbucket.org:texecomworkspace/texecom-cloud.git
      revision: "{{ config.branch }}"
      sshKeyFrom:
        secretKeyRef:
          name: bitbucket-ssh-key
          key: ssh-privatekey

  container:
    image: grafana/k6:latest
    workingDir: /data/repo

  steps:
    - name: Run k6 Load Test
      shell: |
        # Run k6 with multiple output formats
        k6 run tests/load/api-load-test.js \
          --out json=k6-results/detailed.json \
          --summary-export=k6-results/summary.json \
          --vus 10 \
          --duration 30s

        # Generates:
        # - k6-results/summary.json (high-level metrics)
        # - k6-results/detailed.json (all data points)

      artifacts:
        paths:
          - k6-results/**/*
```

**Example k6 Script (tests/load/api-load-test.js):**
```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '10s', target: 10 },  // Ramp up
    { duration: '30s', target: 10 },  // Stay at 10 users
    { duration: '10s', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],  // 95% of requests under 500ms
    http_req_failed: ['rate<0.01'],    // Less than 1% errors
  },
};

export default function() {
  const res = http.get('https://api.example.com/health');
  check(res, {
    'status is 200': (r) => r.status === 200,
    'response time < 500ms': (r) => r.timings.duration < 500,
  });
  sleep(1);
}
```

**What Dashboard Does:**
1. **Download k6 summary.json** - High-level metrics (avg, p95, p99)
2. **Parse metrics** - Extract response times, throughput, error rate
3. **Generate charts (Go)** - Bar charts for percentiles, line charts for throughput
4. **Store in database** - k6_metrics table for historical comparison
5. **Render custom HTML** - Server-side template with embedded SVG charts

**k6 Dashboard View:**
```html
<div class="k6-report">
    <h1>k6 Load Test Results</h1>

    <div class="summary-cards">
        <div class="card">
            <h3>Total Requests</h3>
            <div class="stat">{{.Summary.HttpReqs.Count}}</div>
        </div>
        <div class="card">
            <h3>Average Response Time</h3>
            <div class="stat">{{.Summary.HttpReqDuration.Avg}}ms</div>
        </div>
        <div class="card">
            <h3>P95 Response Time</h3>
            <div class="stat">{{.Summary.HttpReqDuration.P95}}ms</div>
        </div>
        <div class="card">
            <h3>Error Rate</h3>
            <div class="stat">{{.Summary.HttpReqFailed.Rate}}%</div>
        </div>
    </div>

    <!-- Server-generated SVG charts -->
    <div class="charts">
        {{template "k6-response-time-chart" .Charts.ResponseTime}}
        {{template "k6-throughput-chart" .Charts.Throughput}}
        {{template "k6-error-rate-chart" .Charts.ErrorRate}}
    </div>

    <!-- Historical comparison -->
    <div class="comparison">
        <h2>Historical Comparison</h2>
        <table>
            <thead>
                <tr>
                    <th>Date</th>
                    <th>Avg (ms)</th>
                    <th>P95 (ms)</th>
                    <th>Error Rate</th>
                    <th>Trend</th>
                </tr>
            </thead>
            <tbody>
            {{range .HistoricalRuns}}
                <tr>
                    <td>{{.Date}}</td>
                    <td>{{.AvgDuration}}</td>
                    <td>{{.P95Duration}}</td>
                    <td>{{.ErrorRate}}</td>
                    <td>
                        {{if .IsRegression}}
                            <span class="trend-down">📉 Slower</span>
                        {{else}}
                            <span class="trend-up">📈 Faster</span>
                        {{end}}
                    </td>
                </tr>
            {{end}}
            </tbody>
        </table>
    </div>
</div>
```

---

## Technical Architecture

### Backend (Go)

**API Endpoints:**
```
# Dashboard views
GET  /                                      → Dashboard overview
GET  /workflows                             → List all workflows
GET  /workflows/:name                       → Workflow detail + history
GET  /executions/:id                        → Execution detail
GET  /executions/:id/report                 → Serve HTML report (Playwright/Vitest)
GET  /executions/:id/k6-report              → Generate k6 report (Go)
GET  /executions/:id/logs                   → Get execution logs
GET  /executions/:id/artifacts              → List artifacts

# Trend & analytics
GET  /api/v1/trends/pass-rate               → Pass rate chart (SVG)
GET  /api/v1/trends/duration                → Duration trend chart (SVG)
GET  /api/v1/trends/test-volume             → Test volume chart (SVG)
GET  /api/v1/flaky-tests                    → List flaky tests
GET  /api/v1/tests/:name/history            → Single test history (sparkline SVG)

# Search
GET  /search                                → Search executions/tests

# Actions (htmx)
POST /workflows/:name/run                   → Trigger test run
POST /executions/:id/rerun                  → Rerun execution
```

**Key Components:**
```go
// internal/server/server.go
type Server struct {
    testkubeAPI  *testkube.Client
    db           *database.Database
    cache        *cache.Cache
    chartGen     *charts.Generator
    artifactMgr  *artifacts.Manager
    router       *chi.Mux
}

// internal/testkube/client.go
type Client struct {
    baseURL string
    http    *http.Client
}

func (c *Client) GetExecutions(opts ListOptions) ([]Execution, error)
func (c *Client) GetExecution(id string) (*Execution, error)
func (c *Client) GetArtifacts(id string) ([]Artifact, error)
func (c *Client) DownloadArtifact(id, path string) ([]byte, error)

// internal/database/database.go
type Database struct {
    *sql.DB
}

func (db *Database) InsertExecution(exec Execution) error
func (db *Database) InsertTestCase(tc TestCase) error
func (db *Database) InsertK6Metric(metric K6MetricRecord) error
func (db *Database) GetTrends(workflow string, days int) (*TrendData, error)
func (db *Database) GetFlakyTests(threshold float64) ([]FlakyTest, error)

// internal/charts/generator.go
type Generator struct{}

func (g *Generator) PassRateChart(data []DataPoint) string
func (g *Generator) DurationChart(data []DataPoint) string
func (g *Generator) K6ResponseTimeChart(metrics K6Metric) string
func (g *Generator) Sparkline(data []float64) string

// internal/artifacts/manager.go
type Manager struct {
    cacheDir  string
    cacheTTL  time.Duration
}

func (m *Manager) DownloadReport(executionID, pattern string) (string, error)
func (m *Manager) GetCachedReport(executionID string) (string, error)
func (m *Manager) ParsePlaywrightResults(path string) (*PlaywrightResults, error)
func (m *Manager) ParseVitestResults(path string) (*VitestResults, error)
func (m *Manager) ParseK6Summary(path string) (*K6Summary, error)
```

---

### Frontend (htmx + Alpine.js)

**Technology Stack:**
- **htmx**: Server-side rendering, AJAX, WebSockets
- **Alpine.js**: Minimal client-side interactivity (dropdowns, modals, local state)
- **Go templates**: Server-side HTML generation
- **go-echarts**: Server-side SVG chart generation
- **CSS**: Tailwind or custom minimal CSS

**Why htmx?**
- ✅ Minimal JavaScript (better security, performance)
- ✅ Server controls all logic (easier to maintain)
- ✅ Progressive enhancement (works without JS)
- ✅ WebSocket support for real-time logs
- ✅ Better for Go developers (less context switching)

**Interactive Features (htmx):**
```html
<!-- Auto-refresh dashboard every 30s -->
<div hx-get="/api/dashboard-data" hx-trigger="every 30s" hx-swap="innerHTML">
    {{template "dashboard-metrics"}}
</div>

<!-- Load more executions (infinite scroll) -->
<div hx-get="/workflows/{{.Name}}/history?page={{.NextPage}}"
     hx-trigger="intersect once"
     hx-swap="afterend">
    <div class="loading">Loading more...</div>
</div>

<!-- Real-time logs (WebSocket) -->
<pre hx-ext="ws" ws-connect="/executions/{{.ID}}/logs/stream">
    {{.Logs}}
</pre>

<!-- Filter form (live updates) -->
<form hx-get="/workflows/{{.Name}}/history"
      hx-trigger="change, keyup delay:300ms from:input"
      hx-target="#execution-list">
    <input type="text" name="search" placeholder="Search..." />
    <select name="status">
        <option value="">All</option>
        <option value="passed">Passed</option>
        <option value="failed">Failed</option>
    </select>
</form>

<!-- Trigger test run -->
<button hx-post="/workflows/{{.Name}}/run"
        hx-swap="none"
        hx-indicator="#spinner">
    Run Test
</button>
<div id="spinner" class="htmx-indicator">Running...</div>
```

**Alpine.js for Local Interactions:**
```html
<!-- Dropdown menu -->
<div x-data="{ open: false }">
    <button @click="open = !open">Actions ▼</button>
    <div x-show="open" @click.away="open = false">
        <a href="/executions/{{.ID}}/rerun">Rerun</a>
        <a href="/executions/{{.ID}}/export">Export</a>
    </div>
</div>

<!-- Expandable test case details -->
<tr x-data="{ expanded: false }">
    <td @click="expanded = !expanded">{{.TestName}}</td>
    <td>{{.Status}}</td>
    <td>{{.Duration}}</td>
</tr>
<tr x-show="expanded" x-collapse>
    <td colspan="3">
        <pre>{{.ErrorMessage}}</pre>
    </td>
</tr>
```

---

### Data Flow

```
┌──────────┐     ┌──────────────┐     ┌──────────────┐
│ Browser  │────▶│  Dashboard   │────▶│  Testkube   │
│          │     │   (Go/htmx)  │     │  API Server │
│          │◀────│              │◀────│             │
└──────────┘     └───────┬──────┘     └──────────────┘
                         │                     │
                         │                     ▼
                         │              ┌──────────────┐
                         │              │  MinIO       │
                         └─────────────▶│  (artifacts) │
                           download     └──────────────┘
                           artifacts

                         │
                         ▼
                  ┌──────────────┐
                  │ PostgreSQL   │
                  │ (Metrics DB) │
                  └──────────────┘
```

**Flow for viewing a Playwright report:**
1. User clicks "View Report" for execution `abc123`
2. Dashboard checks local cache
3. If not cached:
   - Download `playwright-report/**/*` from Testkube API
   - Cache locally (10-minute TTL)
4. Serve `index.html` + assets
5. Browser renders interactive Playwright report (includes trace viewer!)

**Flow for viewing k6 report:**
1. User clicks "View Report" for k6 execution `xyz789`
2. Dashboard downloads `k6-results/summary.json`
3. Parse JSON in Go
4. Generate SVG charts with go-echarts
5. Render Go template with charts embedded
6. Serve HTML to browser

**Flow for historical trends:**
1. Background job runs after each execution completes
2. Downloads artifacts (JSON/XML results)
3. Parses test cases and metrics
4. Inserts into PostgreSQL
5. When user views dashboard:
   - Query database for trends (last 30 days)
   - Generate SVG charts server-side
   - Embed in HTML template
   - Serve to browser

---

## Storage Strategy

### Artifact Storage (Testkube MinIO)

**Current Configuration:**
- **Endpoint:** `testkube-minio-service-testkube:9000`
- **Bucket:** `testkube-artifacts`
- **Expiration:** `STORAGE_EXPIRATION=0` (infinite retention)
- **Compression:** `COMPRESSARTIFACTS=true` (gzip)
- **Credentials:** `minio/minio123`

**Artifact Organization:**
```
testkube-artifacts/
└── executions/
    └── {execution-id}/
        ├── playwright-report/
        │   ├── index.html
        │   ├── data/*.webm (videos)
        │   ├── data/*.png (screenshots)
        │   └── trace/*.zip (traces)
        ├── test-results/
        │   ├── results.json (Playwright JSON)
        │   ├── junit.xml (Vitest JUnit)
        │   ├── results.json (Vitest JSON)
        │   └── html/index.html (Vitest HTML)
        ├── coverage/
        │   └── index.html
        ├── k6-results/
        │   ├── summary.json
        │   └── detailed.json
        └── .testkube/
            └── metrics/*.influx
```

**Retention Policy:**
- **Default:** Keep forever (`STORAGE_EXPIRATION=0`)
- **Optional:** Custom retention in dashboard settings
  - Delete artifacts older than N days
  - Keep last M executions per workflow
  - Keep failures longer than passes

**Data Volume Estimates:**
- **Per Playwright execution:** 50-200 MB (videos are large)
- **Per Vitest execution:** 1-5 MB
- **Per k6 execution:** 1-50 MB (depends on duration)
- **Current 301 executions:** ~30-60 GB
- **Annual growth:** ~500 GB (2 executions/day average)

---

### Metrics Database (PostgreSQL)

**Why PostgreSQL?**
- ✅ Better for complex queries (trends, aggregations)
- ✅ JSONB support for flexible metadata
- ✅ Full-text search
- ✅ Industry standard, well-supported

**Alternative: SQLite**
- ✅ Zero configuration, embedded
- ✅ Perfect for small-medium scale
- ⚠️ Limited concurrency
- Use if: Single dashboard instance, <1000 executions/month

**Database Size Estimates:**
- **Per execution:** ~1 KB metadata
- **Per test case:** ~500 bytes
- **Per k6 metric:** ~200 bytes
- **1 year of data:** ~50-100 MB (very small!)

**Backup Strategy:**
- Daily PostgreSQL dumps
- Store in MinIO or external backup
- 90-day retention for backups

---

## Authentication & Security

### Current State
- Dashboard deployed with K8s ServiceAccount (read-only RBAC)
- No authentication on dashboard itself
- Protected by K8s network policies

### Recommended Enhancement

**Option 1: Basic Auth (Simple)**
```go
func (s *Server) basicAuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        username := os.Getenv("DASHBOARD_USERNAME")
        password := os.Getenv("DASHBOARD_PASSWORD")

        if username == "" {
            // Auth disabled
            next.ServeHTTP(w, r)
            return
        }

        user, pass, ok := r.BasicAuth()
        if !ok || user != username || pass != password {
            w.Header().Set("WWW-Authenticate", `Basic realm="Testkube Dashboard"`)
            http.Error(w, "Unauthorized", 401)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

**Option 2: OAuth2/OIDC (Enterprise)**
- Integrate with corporate SSO (Google, Okta, Azure AD)
- Use existing identity provider
- Group-based permissions

**Option 3: Read-Only vs Admin Roles**
```go
type User struct {
    Username string
    Role     string // "viewer", "runner", "admin"
}

// Viewers: Can view reports, no actions
// Runners: Can trigger test runs
// Admins: Can modify workflows, settings
```

---

## Deployment & Operations

### Resource Requirements

**Dashboard Pod:**
```yaml
resources:
  requests:
    cpu: 200m
    memory: 512Mi
    ephemeral-storage: 2Gi  # For artifact caching
  limits:
    cpu: 1000m
    memory: 2Gi
    ephemeral-storage: 5Gi
```

**PostgreSQL Pod:**
```yaml
resources:
  requests:
    cpu: 100m
    memory: 256Mi
  limits:
    cpu: 500m
    memory: 1Gi

persistence:
  size: 10Gi  # Plenty for metrics
```

### Dependencies
- ✅ Testkube API server (already running)
- ✅ Testkube MinIO (already running)
- ✅ PostgreSQL (new - for metrics/trends)
- ❌ ~~Allure Server~~ (removed!)
- ❌ ~~Java runtime~~ (removed!)

### Monitoring

**Health Endpoints:**
```go
// /healthz - Liveness probe
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(200)
    w.Write([]byte("OK"))
}

// /readyz - Readiness probe
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
    // Check DB connection
    if err := s.db.Ping(); err != nil {
        http.Error(w, "Database unavailable", 503)
        return
    }

    // Check Testkube API
    if _, err := s.testkubeAPI.GetExecutions(ListOptions{PageSize: 1}); err != nil {
        http.Error(w, "Testkube API unavailable", 503)
        return
    }

    w.WriteHeader(200)
    w.Write([]byte("Ready"))
}

// /metrics - Prometheus metrics
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
    // Expose:
    // - artifact_downloads_total
    // - chart_generation_duration_seconds
    // - cache_hit_rate
    // - database_query_duration_seconds
}
```

**Prometheus Metrics:**
```
# Cache performance
testkube_dashboard_cache_hits_total{type="playwright_report"}
testkube_dashboard_cache_misses_total{type="playwright_report"}

# Chart generation
testkube_dashboard_chart_generation_duration_seconds{type="pass_rate"}

# Artifact downloads
testkube_dashboard_artifact_download_bytes_total
testkube_dashboard_artifact_download_duration_seconds

# Database queries
testkube_dashboard_db_query_duration_seconds{query="get_trends"}
```

### Performance Considerations

**Artifact Caching:**
- Cache downloaded reports for 10 minutes
- Use local filesystem (ephemeral storage)
- LRU eviction when storage is low
- Shared volume for multi-replica deployments (optional)

**Chart Generation:**
- Generate charts on-demand (not pre-generated)
- Cache chart SVGs for 5 minutes
- Most charts render in <100ms

**Database Queries:**
- Index on `(workflow_name, started_at DESC)`
- Index on `test_name` for flaky test queries
- Use EXPLAIN ANALYZE for slow queries
- Connection pool size: 10-20

**Concurrent Requests:**
- Handle 50+ concurrent users easily
- Scale horizontally with multiple replicas
- PostgreSQL can handle 100+ connections

---

## Migration Path

### Phase 1: Basic Execution History ✅ DONE
- ✅ Dashboard deployed
- ✅ Reading CRDs only (ephemeral data - identified as limitation)

### Phase 2: API Integration (1-2 days)
- Modify backend to query Testkube API instead of CRDs
- Display full execution history (301+ executions)
- Show execution metadata, logs, artifact lists
- **Deliverable:** Working dashboard with history, no reports yet

### Phase 3: PostgreSQL + Historical Tracking (1 day)
- Deploy PostgreSQL
- Create database schema
- Implement background job to parse artifacts
- Store metrics in database
- **Deliverable:** Historical data being collected

### Phase 4: Playwright Report Integration (1 day)
- Update TestWorkflow to save `playwright-report/**/*`
- Implement artifact download + caching
- Serve Playwright HTML reports directly
- **Deliverable:** Full Playwright reports viewable

### Phase 5: Vitest Report Integration (1 day)
- Add Vitest HTML reporter to config
- Update TestWorkflow to save `test-results/html/**/*`
- Implement Vitest report serving
- Parse JUnit XML for metrics
- **Deliverable:** Vitest reports + unit test tracking

### Phase 6: k6 Integration (2 days)
- Create k6 TestWorkflow
- Implement k6 summary.json parser
- Build Go-based chart generation (go-echarts)
- Create k6 report template
- **Deliverable:** k6 performance testing integrated

### Phase 7: Trend Charts (2 days)
- Implement server-side chart generation (go-echarts)
- Create trend queries (pass rate, duration, volume)
- Build flaky test detection
- Add sparklines to test case history
- **Deliverable:** Historical insights and trends

### Phase 8: Allure Server Deprecation (1 week)
- Run dual-storage (MinIO + Allure Server) for safety
- Monitor dashboard usage and stability
- Update all TestWorkflows to stop uploading to Allure Server
- Scale down and remove Allure Server deployment
- **Deliverable:** Simplified architecture, no Java

### Phase 9: Polish & Advanced Features (3-5 days)
- Search functionality
- Saved filter views
- Performance regression detection
- Email/Slack notifications for failures
- **Deliverable:** Production-ready dashboard

**Total Estimated Effort:** 12-18 days for full implementation + migration

---

## Cost Comparison

| Solution | Monthly Cost | Annual Cost | Features | Trade-offs |
|----------|--------------|-------------|----------|------------|
| **Official Testkube Cloud** | $400 | $4,800 | Full SaaS, support, cloud storage | Vendor lock-in, recurring cost |
| **Separate Allure Server** | $0 | $0 | Detailed reports | Java dependency, extra infra |
| **This Unified Dashboard** | $0 | $0 | Complete solution, multi-framework, trends | Self-hosted, initial dev effort |

**Total Savings:** $4,800/year + reduced operational overhead

**Additional Benefits:**
- ✅ No Java runtime (150MB+ image bloat removed)
- ✅ Unified retention policy (one storage system)
- ✅ Historical trend tracking (not available in Allure)
- ✅ Multi-framework support (Playwright + Vitest + k6)
- ✅ Flaky test detection (automatic)
- ✅ Performance regression alerts (k6)
- ✅ Single authentication system
- ✅ Reduced complexity (one service instead of two)
- ✅ Better integration (everything in one UI)

---

## Technical Dependencies

### Required Software
- **Go 1.24+** (dashboard backend)
- **Docker** (containerization)
- **Kubernetes** (deployment platform)
- **PostgreSQL 15+** (metrics database)
- ~~**Java 11+**~~ (REMOVED!)
- ~~**Allure CLI**~~ (REMOVED!)

### Go Libraries
```go
require (
    github.com/go-chi/chi/v5 v5.2.3           // HTTP router
    github.com/lib/pq v1.10.9                  // PostgreSQL driver
    github.com/patrickmn/go-cache v2.1.0       // In-memory cache
    github.com/go-echarts/go-echarts/v2 v2.3.3 // Chart generation
    k8s.io/client-go v0.32.1                   // K8s client (optional)
)
```

### Frontend Libraries (CDN)
```html
<script src="https://unpkg.com/htmx.org@1.9.10"></script>
<script src="https://unpkg.com/alpinejs@3.13.3"></script>
<!-- NO Chart.js - using server-side go-echarts instead! -->
```

---

## Chart Generation Approach (htmx-Compatible)

### Server-Side Chart Generation (Recommended)

**Library: go-echarts**
- Pure Go, no JavaScript dependencies
- Generates interactive SVG/HTML charts
- Rich chart types: line, bar, pie, scatter, heatmap
- Apache ECharts under the hood (mature, feature-rich)
- **htmx-friendly:** Just serve the SVG

**Example Implementation:**
```go
import (
    "github.com/go-echarts/go-echarts/v2/charts"
    "github.com/go-echarts/go-echarts/v2/opts"
    "github.com/go-echarts/go-echarts/v2/types"
)

func (s *Server) generatePassRateTrendChart(data []TrendDataPoint) string {
    line := charts.NewLine()

    // Global options
    line.SetGlobalOptions(
        charts.WithInitializationOpts(opts.Initialization{
            Width:  "800px",
            Height: "400px",
            Theme:  types.ThemeWesteros,
        }),
        charts.WithTitleOpts(opts.Title{
            Title:    "Pass Rate Trend (30 Days)",
            Subtitle: "Daily pass rate percentage",
        }),
        charts.WithTooltipOpts(opts.Tooltip{
            Show:      true,
            Trigger:   "axis",
        }),
        charts.WithLegendOpts(opts.Legend{
            Show: true,
            Top:  "30px",
        }),
        charts.WithToolboxOpts(opts.Toolbox{
            Show: true,
            Feature: &opts.ToolBoxFeature{
                SaveAsImage: &opts.ToolBoxFeatureSaveAsImage{
                    Show:  true,
                    Title: "Save",
                },
            },
        }),
    )

    // X-axis (dates)
    xAxis := make([]string, len(data))
    for i, dp := range data {
        xAxis[i] = dp.Date.Format("Jan 02")
    }

    // Y-axis (pass rate)
    yAxis := make([]opts.LineData, len(data))
    for i, dp := range data {
        yAxis[i] = opts.LineData{
            Value: dp.PassRate,
            Symbol: "circle",
            SymbolSize: 8,
        }
    }

    line.SetXAxis(xAxis).
         AddSeries("Pass Rate %", yAxis).
         SetSeriesOptions(
             charts.WithLineChartOpts(opts.LineChart{
                 Smooth: true,
                 ShowSymbol: true,
             }),
             charts.WithMarkLineNameTypeItemOpts(opts.MarkLineNameTypeItem{
                 Name: "Target (95%)",
                 YAxis: 95.0,
             }),
             charts.WithAreaStyleOpts(opts.AreaStyle{
                 Opacity: 0.3,
             }),
         )

    // Render to HTML snippet
    var buf bytes.Buffer
    line.Render(&buf)
    return buf.String()
}
```

**Usage in Go Template:**
```html
<div class="chart-container">
    {{.PassRateTrendChart | safeHTML}}
</div>
```

**Benefits:**
- ✅ **Pure htmx/Go stack** - no JavaScript frameworks
- ✅ **Interactive** - tooltips, zoom, pan (built-in via ECharts)
- ✅ **Responsive** - adapts to screen size
- ✅ **Exportable** - built-in save-as-image
- ✅ **Fast** - generated server-side, cached
- ✅ **SEO-friendly** - server-rendered HTML

**Chart Types Needed:**
1. **Line Charts** - Pass rate trends, duration trends
2. **Bar Charts** - Test volume by day, k6 percentiles
3. **Sparklines** - Mini charts in table rows (pass/fail history)
4. **Heatmaps** - Test stability matrix (which tests fail when)
5. **Pie Charts** - Test distribution by status

### Alternative: Simple SVG Generation (Minimal Dependency)

For very simple charts (sparklines, small bar charts), generate raw SVG:

```go
func generateSparkline(values []float64, width, height int) string {
    if len(values) == 0 {
        return ""
    }

    // Normalize values to height
    min, max := values[0], values[0]
    for _, v := range values {
        if v < min { min = v }
        if v > max { max = v }
    }

    points := make([]string, len(values))
    for i, v := range values {
        x := float64(i) * float64(width) / float64(len(values)-1)
        y := float64(height) - ((v - min) / (max - min) * float64(height))
        points[i] = fmt.Sprintf("%.1f,%.1f", x, y)
    }

    polyline := strings.Join(points, " ")

    return fmt.Sprintf(`
        <svg width="%d" height="%d" class="sparkline">
            <polyline points="%s"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"/>
        </svg>
    `, width, height, polyline)
}
```

**Usage:**
```html
<td>
    {{sparkline .PassRateHistory 100 30 | safeHTML}}
</td>
```

**When to use:**
- Simple sparklines in tables
- Very basic charts where go-echarts is overkill
- Minimal dependency preference

---

## Open Questions & Decisions Needed

### 1. Chart Library Choice
**Question:** go-echarts (feature-rich) vs. custom SVG (minimal)?

**Recommendation:** **go-echarts**
- Pros: Professional charts, interactive, saves development time
- Cons: +2MB dependency
- Decision: Use go-echarts for main charts, custom SVG for sparklines

### 2. Database Choice
**Question:** PostgreSQL (robust) vs. SQLite (simple)?

**Recommendation:** **PostgreSQL**
- Pros: Better for concurrent access, full-text search, JSONB
- Cons: Extra deployment complexity
- Decision: Start with PostgreSQL, document SQLite as option for small deployments

### 3. Cache Storage
**Question:** In-memory vs. Persistent Volume?

**Recommendation:** **In-memory (ephemeral storage) for MVP**
- Pros: Simple, fast
- Cons: Lost on pod restart
- Decision: Use ephemeral storage, upgrade to PV if cache misses cause issues

### 4. Authentication
**Question:** Basic Auth vs. OAuth2?

**Recommendation:** **Basic Auth for MVP, OAuth2 as future enhancement**
- Most teams: Basic Auth is sufficient
- Enterprise: Add OAuth2 later
- Decision: Ship with Basic Auth, document OAuth2 integration path

### 5. Vitest HTML Reporter
**Question:** Is Vitest HTML reporter production-ready?

**Note:** Vitest HTML reporter is relatively new (added in Vitest 1.0)
- **Test it first:** Run locally, verify it generates usable reports
- **Fallback:** Parse JUnit XML and generate custom HTML with Go templates
- **Decision:** Try HTML reporter, fallback to custom if needed

---

## Success Metrics

### Technical Metrics
- **Chart Generation Time:** < 500ms (p95)
- **Cache Hit Rate:** > 70% (after warm-up period)
- **Page Load Time:** < 2 seconds (dashboard overview)
- **Database Query Time:** < 100ms (p95)
- **Artifact Download Time:** < 5 seconds (Playwright report)
- **Uptime:** > 99.5%

### Business Metrics
- **Team Adoption:** > 90% of test reviews done via dashboard
- **Allure Server Usage:** 0% (deprecated)
- **Cost Savings:** $4,800/year (vs. Testkube Cloud)
- **Time Saved:** 20 min/day (faster report access, no Allure Server)
- **Flaky Test Reduction:** Identify and fix top 10 flaky tests in 30 days

### User Experience Metrics
- **Report Access Time:** < 10 seconds from "View Report" click
- **Search Response Time:** < 1 second
- **Historical Trends:** 30-day trends loaded in < 2 seconds

---

## Conclusion

This unified dashboard architecture provides a **complete, multi-framework testing solution** by:

1. **Eliminating Java Dependency:** No Allure CLI, no 150MB+ image bloat
2. **Consolidating Storage:** All test data in Testkube MinIO
3. **Adding Intelligence:** Historical trends, flaky test detection, regression alerts
4. **Multi-Framework Support:** Playwright + Vitest + k6 in one unified UI
5. **htmx-First Design:** Server-side rendering, minimal JavaScript, fast and maintainable
6. **Zero Licensing Cost:** Avoid $4,800/year Testkube Cloud subscription
7. **Simplified Operations:** Remove Allure Server, reduce complexity

**Key Technical Decisions:**
- ✅ **htmx + Go** for all rendering (not Chart.js)
- ✅ **go-echarts** for server-side chart generation
- ✅ **PostgreSQL** for historical metrics
- ✅ **Playwright HTML reports** served directly (no Java!)
- ✅ **Custom k6 report generation** in Go
- ✅ **Background jobs** parse artifacts and build trends

The approach is **technically sound**, leverages existing infrastructure (Testkube + MinIO), and provides **95% of commercial solution value** at **0% of the cost**. The 12-18 day implementation effort pays for itself immediately through eliminated licensing costs ($400/month) and reduced operational overhead (no Java, no Allure Server).

**The key insight:** Test frameworks already generate great HTML reports (Playwright, Vitest). We just need to save them to MinIO, serve them with Go, and add historical intelligence on top. No Java required.
