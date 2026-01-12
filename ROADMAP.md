# Testkube Dashboard Roadmap

This project aims to provide a lightweight, high-performance dashboard for Testkube running in Kubernetes, utilizing Go and htmx. The goal is to achieve feature parity with the existing dashboard while maintaining a significantly lower resource footprint and faster load times.

## Guiding Principles
- **Early Value**: Deliver a functional read-only view first, then add interactivity.
- **Low Resource Usage**: Minimal footprint to run alongside other workloads.
- **Performance**: Fast rendering and updates using htmx and server-side rendering.

## Roadmap

### Phase 1: MVP - List Tests (Completed)
- [x] Initialize Go project structure.
- [x] Set up HTTP server and basic routing.
- [x] Implement Kubernetes client (mockable for dev).
- [x] Create UI to list Testkube Tests (CRDs).
- [x] Integrate htmx for dynamic content loading.
- [x] Containerize (Dockerfile).

### Phase 2: Test Execution & Inspection (Completed)
- [x] View Test details and configuration.
- [x] View recent Test Executions.
- [x] Trigger manual test runs.
- [ ] View execution logs (currently placeholder).

### Phase 2.5: Fixes and Refinements (Immediate)
- [ ] **Fix Logs:** Implement proper log streaming in the `handleExecutionLogs` function.
- [ ] **Refactor History:** Rework the `handleWorkflowHistory` to use the template system for consistency.
- [ ] **Conditional User Gen:** Initialize the `userGen` only when a real database is configured.
- [ ] **Improve Error Handling:** Display clear error messages on the dashboard when data cannot be fetched.
- [ ] **Add Unit Tests:** Introduce unit tests for the server handlers and other critical components.

### Phase 3: Real-time Visibility & Insights (Upcoming)
- [ ] **Dashboard Home**: Summary view of cluster health, pass/fail rates, and recent activity.
- [ ] **Real-time Updates**: Implement Server-Sent Events (SSE) to update status and stream logs live without page refreshes.
- [ ] **Artifacts Browser**: View and download test artifacts (screenshots, HTML reports, generic files) directly in the browser.
- [ ] **TestWorkflows Support**: Full support for the new TestWorkflow CRD (visualization of steps, DAGs).
- [ ] **Results Table:** Display test results in a filterable and sortable table on the dashboard.
- [ ] **Individual Results Pages:** Create dedicated pages for each test result, showing detailed information.
- [ ] **Link to External UIs:** Where applicable, link out to the tool's own UI (e.g., for detailed reports).

### Phase 4: Full Management & Parity
- [ ] **Test Creation/Editing**: Forms to create and update Tests, Test Suites, and TestWorkflows.
- [ ] **Test Suites**: Manage and execute collections of tests.
- [ ] **Executors Management**: View and configure available executors (Postman, Cypress, K6, etc.).
- [ ] **Sources & Triggers**: Manage Git sources and Test Triggers (cron, K8s events).

### Phase 5: Enterprise-Ready Features
- [ ] **Authentication & RBAC**: Integrate with OIDC providers and respect Kubernetes RBAC for view/edit permissions.
- [ ] **Metrics & Observability**: Expose Prometheus metrics for the dashboard itself and visualize test metrics.
- [ ] **Multi-Environment**: Context switching between different namespaces or clusters.
- [ ] **Dark Mode & Theming**: Polished UI with theme support.

### Allure Integration
- [ ] **Optional Allure Support:** Investigate how to optionally integrate with Allure.
- [ ] **Allure Report Linking:** If an Allure report is available as an artifact, provide a direct link to it.
- [ ] **Allure Metrics:** Explore the possibility of parsing Allure results to extract metrics and display them in the dashboard.
