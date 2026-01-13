# Testkube Dashboard Roadmap

This roadmap outlines the development plan for the Testkube Dashboard, a lightweight, high-performance UI focused on engineering and operational concerns. The goal is to provide deep insights into test results, historical trends, and performance metrics, leveraging a Go backend and a minimal htmx-based frontend.

This plan is heavily influenced by the detailed technical breakdown in `FEATURE_SPECIFICATION.md`.

## Guiding Principles
- **Engineering First**: Prioritize features that help developers and operators understand test health, identify regressions, and diagnose failures.
- **High Performance**: Maintain a low resource footprint with fast server-side rendering.
- **Unified View**: Consolidate reports and metrics from multiple test frameworks (Playwright, Vitest, k6, etc.) into a single interface.
- **Historical Intelligence**: Go beyond simple execution lists to provide trend analysis, flaky test detection, and performance tracking.

---

### **Phase 1: Core Read-Only Functionality (Partially Completed)**
*Objective: Establish a stable foundation for viewing live test data from the Testkube API.*

- [x] List TestWorkflows and Executions fetched directly from the Testkube API.
- [x] View basic execution details (status, start/end time, duration).
- [ ] **Implement Log Streaming**: Display real-time logs for running executions by streaming from the Testkube API.
- [ ] **View Artifacts**: Show a list of all files and artifacts associated with an execution.
- [ ] **Refine UI & Error Handling**: Ensure the UI is clean and provides clear error messages when the Testkube API is unavailable.

---

### **Phase 2: Historical Data & Analytics Backend**
*Objective: Create a persistent data store to enable historical trend analysis and advanced insights.*

- [ ] **Introduce PostgreSQL Backend**: Deploy a PostgreSQL database to store aggregated test metrics and historical data.
- [ ] **Define Database Schema**: Create a robust schema for tracking executions, individual test cases, and performance metrics (e.g., from k6).
- [ ] **Implement Artifact Parsing Worker**:
    - Create a background job that triggers after an execution completes.
    - The worker will download structured test results (e.g., Playwright JSON, Vitest JUnit XML, k6 `summary.json`) from Testkube's MinIO storage.
    - It will then parse these files and populate the PostgreSQL database with detailed, queryable data.

---

### **Phase 3: Integrated Multi-Framework Report Viewers**
*Objective: Eliminate the need for separate report servers by rendering detailed test reports directly within the dashboard.*

- [ ] **Playwright Report Viewer**:
    - Natively serve the `playwright-report/index.html` and its assets (videos, traces) from the execution's artifacts.
    - Implement artifact caching on the dashboard server for fast, repeated access.
- [ ] **Vitest Report Viewer**:
    - Natively serve the Vitest HTML report (`test-results/html/index.html`).
    - Provide a link to the browsable code coverage report (`coverage/index.html`).
- [ ] **k6 Performance Report Generator**:
    - Parse the `k6/summary.json` artifact.
    - Generate a dedicated, server-rendered k6 report page using Go.
    - Display key performance indicators (KPIs) like requests per second, p95/p99 latency, and error rates.
    - Use `go-echarts` to embed server-side generated SVG charts (e.g., response time distribution).

---

### **Phase 4: Analytics UI & Insight Visualization**
*Objective: Surface the historical data collected in Phase 2 through insightful charts and analytics.*

- [ ] **Main Dashboard View**:
    - Create a homepage that provides a high-level overview of test health, queried from the PostgreSQL database.
    - Display key metrics like overall pass rate, average test duration, and a list of recent failures.
    - Render historical trend charts (e.g., pass rate over time, duration trends) using `go-echarts`.
- [ ] **Flaky Test Detection**:
    - Implement the flaky test detection algorithm as specified in the feature document, using historical data.
    - Highlight the top flaky tests on the main dashboard to prioritize fixes.
- [ ] **Enhanced History Views**:
    - Augment the workflow and execution history lists with historical trend data (e.g., sparkline charts showing pass/fail history).
    - Add powerful filtering by status, date range, and branch.
- [ ] **Detailed Execution View**:
    - On the execution detail page, show a breakdown of all individual test cases (passed, failed, skipped) from the database.
    - For each test case, display a sparkline of its recent pass/fail history to spot inconsistencies.

---

### **Phase 5: Operational Excellence & Usability**
*Objective: Make the dashboard robust, easy to operate, and more interactive for daily use.*

- [ ] **Observability**:
    - Expose key operational metrics via a `/metrics` endpoint for Prometheus scraping.
    - Implement `/healthz` and `/readyz` endpoints for reliable Kubernetes liveness and readiness probes.
- [ ] **Global Search**: Implement a search feature to quickly find workflows, specific executions, or individual test cases across all history.
- [ ] **Basic Test Management**:
    - Add a "Run" button to manually trigger a TestWorkflow.
    - Add a "Re-run" button to re-execute a failed run.
- [ ] **Multi-Environment Support**: Allow the dashboard to be configured to point to different Testkube instances (e.g., staging vs. production) via environment variables.

---

### **De-prioritized / Future Considerations**

The following features are considered lower priority to maintain focus on the core engineering and operational goals.

- **Complex Authentication & RBAC**: Advanced OIDC integration is deferred. The dashboard should be protected by network policies or, if necessary, basic authentication.
- **Full CRUD for Testkube Resources**: Creating and editing tests, suites, and triggers from the UI is out of scope. These should be managed via a GitOps workflow.
- **UI Theming**: Features like Dark Mode are considered non-essential for the initial versions.