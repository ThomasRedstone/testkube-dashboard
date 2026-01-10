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
- [x] View execution logs.
- [x] Trigger manual test runs.

### Phase 3: Real-time Visibility & Insights (Upcoming)
- [ ] **Dashboard Home**: Summary view of cluster health, pass/fail rates, and recent activity.
- [ ] **Real-time Updates**: Implement Server-Sent Events (SSE) to update status and stream logs live without page refreshes.
- [ ] **Artifacts Browser**: View and download test artifacts (screenshots, HTML reports, generic files) directly in the browser.
- [ ] **TestWorkflows Support**: Full support for the new TestWorkflow CRD (visualization of steps, DAGs).

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
