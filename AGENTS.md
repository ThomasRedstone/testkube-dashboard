# Agent Guide - Testkube Dashboard

Welcome, Agent. This guide explains how to work with the Testkube Dashboard codebase, particularly when a real Kubernetes environment is unavailable.

## Simulation Mode (Development Without K8s)

The project includes a robust simulation mode that allows you to develop and test all features without a Kubernetes cluster or the Testkube API.

### Enabling Simulation
Set the `USE_MOCK` environment variable to `true`:

```bash
export USE_MOCK="true"
go run ./cmd/server/main.go
```

### Simulated Behaviors
- **Lifecycle**: Executions transition from `queued` -> `running` -> `passed/failed` over several seconds.
- **Logs**: Real-time logs are generated during the simulation and can be streamed.
- **Artifacts**: Mock artifacts (JSON, HTML, XML) are provided for testing report parsers.
- **History**: Pre-populated with diverse workflow types (Playwright, Vitest, k6, etc.).

## Project Structure

- `cmd/server/`: Entry point for the Go application.
- `internal/testkube/`: Testkube API clients. `mock_client.go` contains the simulation logic.
- `internal/database/`: Persistence layer. `mock_database.go` provides in-memory storage for development.
- `web/templates/`: htmx-powered Go templates for the UI.

## Working with htmx

This project uses **htmx** for interactivity. 
- Avoid writing custom JavaScript. 
- Use `hx-*` attributes in templates to handle AJAX requests and partial DOM updates.
- Keep logic in the Go backend; the server should return HTML fragments.

## Codebase Context

When helping with this project, prioritize:
1. **Engineering Concerns**: Visibility, diagnostics, and performance.
2. **Minimalism**: Low resource usage and zero-dependency frontend where possible.
3. **htmx Patterns**: idiomatic server-side state management.

## Testing

Run unit tests:
```bash
go test ./...
```

For e2e tests (requires python/pytest):
```bash
pytest tests/e2e/
```
