# Testkube Dashboard Roadmap

This project aims to provide a lightweight, high-performance dashboard for Testkube running in Kubernetes, utilizing Go and htmx.

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

### Phase 3: Dashboard & Real-time Updates
- [ ] Dashboard summary view (Pass/Fail rates, recent activity).
- [ ] Server-Sent Events (SSE) for real-time status updates of running tests.
- [ ] Improved UI/UX (CSS framework, responsive design).

### Phase 4: Management & Advanced Features
- [ ] Create and Edit Tests via UI.
- [ ] Test Suites management.
- [ ] Scheduling and triggers configuration.
