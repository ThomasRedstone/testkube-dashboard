package server

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/testkube/dashboard/internal/database"
	"github.com/testkube/dashboard/internal/testkube"
)

type Server struct {
	api       testkube.Client
	db        database.Database
	templates *template.Template
}

func NewServer(api testkube.Client, db database.Database) *Server {
	// Load templates
	templatesDir := "web/templates"
	templates := template.Must(template.ParseGlob(filepath.Join(templatesDir, "*.html")))

	return &Server{
		api:       api,
		db:        db,
		templates: templates,
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	// Static files
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// Main routes
	r.Get("/", s.handleDashboard)
	r.Get("/workflows", s.handleWorkflowList)
	r.Get("/workflows/{name}", s.handleWorkflowDetail)
	r.Post("/workflows/{name}/run", s.handleRunWorkflow)
	r.Get("/workflows/{name}/history", s.handleWorkflowHistory)
	r.Get("/executions/{id}", s.handleExecutionDetail)
	r.Get("/executions/{id}/report", s.handleExecutionReport)
	r.Get("/executions/{id}/logs", s.handleExecutionLogs)

	// API routes
	r.Get("/api/v1/flaky-tests", s.handleFlakyTestsAPI)

	return r
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	// Get trend data from database
	trends, err := s.db.GetTrends(7)
	if err != nil {
		log.Printf("Error getting trends: %v", err)
	}

	// Get recent failures
	executions, err := s.api.GetExecutions(testkube.ListOptions{
		Status:   "failed",
		PageSize: 10,
	})
	if err != nil {
		log.Printf("Error getting executions: %v", err)
	}

	// Get flaky tests
	flakyTests, err := s.db.GetFlakyTests(0.1)
	if err != nil {
		log.Printf("Error getting flaky tests: %v", err)
	}

	data := map[string]interface{}{
		"PassRate":       0,
		"PassRateTrend":  "0%",
		"AvgDuration":    "0s",
		"DurationTrend":  "0%",
		"TotalTests":     0,
		"FlakyTests":     flakyTests,
		"RecentFailures": executions,
		"PassRateChart":  template.HTML(""),
		"DurationChart":  template.HTML(""),
	}

	if trends != nil {
		data["PassRate"] = int(trends.CurrentPassRate * 100)
		data["PassRateTrend"] = trends.PassRateChange
		data["AvgDuration"] = trends.AvgDuration.String()
		data["DurationTrend"] = trends.DurationChange
	}

	s.render(w, "layout", data)
}

func (s *Server) handleWorkflowList(w http.ResponseWriter, r *http.Request) {
	workflows, err := s.api.GetWorkflows()
	if err != nil {
		log.Printf("Error getting workflows: %v", err)
		http.Error(w, "Failed to load workflows", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Workflows": workflows,
	}

	s.render(w, "layout", data)
}

func (s *Server) handleWorkflowDetail(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	workflow, err := s.api.GetWorkflow(name)
	if err != nil {
		log.Printf("Error getting workflow: %v", err)
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}

	executions, err := s.api.GetExecutions(testkube.ListOptions{
		Workflow: name,
		PageSize: 20,
	})
	if err != nil {
		log.Printf("Error getting executions: %v", err)
	}

	data := map[string]interface{}{
		"Name":          workflow.Name,
		"Executions":    executions,
		"PassRateChart": template.HTML(""),
	}

	s.render(w, "layout", data)
}

func (s *Server) handleRunWorkflow(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	exec, err := s.api.RunWorkflow(name)
	if err != nil {
		log.Printf("Error running workflow %s: %v", name, err)
		http.Error(w, "Failed to run workflow", http.StatusInternalServerError)
		return
	}

	log.Printf("Started execution %s for workflow %s", exec.ID, name)

	// Return success with HX-Trigger to show notification
	w.Header().Set("HX-Trigger", `{"showMessage": "Workflow started successfully"}`)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleWorkflowHistory(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	// page := r.URL.Query().Get("page")

	executions, err := s.api.GetExecutions(testkube.ListOptions{
		Workflow: name,
		PageSize: 20,
	})
	if err != nil {
		log.Printf("Error getting executions: %v", err)
		http.Error(w, "Failed to load history", http.StatusInternalServerError)
		return
	}

	// Return just the table rows for HTMX partial
	for _, exec := range executions {
		w.Write([]byte(`<tr>
			<td><a href="/executions/` + exec.ID + `">` + exec.Name + `</a></td>
			<td><span class="status status-` + exec.Status + `">` + exec.Status + `</span></td>
			<td>` + exec.StartTime.Format("2006-01-02 15:04") + `</td>
			<td>` + exec.Duration.String() + `</td>
			<td>` + exec.Branch + `</td>
			<td>
				<a href="/executions/` + exec.ID + `" class="btn-secondary">Details</a>
			</td>
		</tr>`))
	}
}

func (s *Server) handleExecutionDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	exec, err := s.api.GetExecution(id)
	if err != nil {
		log.Printf("Error getting execution: %v", err)
		http.Error(w, "Execution not found", http.StatusNotFound)
		return
	}

	testCases, err := s.db.GetExecutionMetrics(id)
	if err != nil {
		log.Printf("Error getting test cases: %v", err)
	}

	data := map[string]interface{}{
		"Execution": exec,
		"TestCases": testCases,
	}

	s.render(w, "layout", data)
}

func (s *Server) handleExecutionReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Try to get the Playwright report artifact
	artifacts, err := s.api.GetArtifacts(id)
	if err != nil {
		log.Printf("Error getting artifacts: %v", err)
		http.Error(w, "Failed to load report", http.StatusInternalServerError)
		return
	}

	// Look for HTML report
	for _, artifact := range artifacts {
		if filepath.Ext(artifact.Name) == ".html" {
			data, err := s.api.DownloadArtifact(id, artifact.Path)
			if err != nil {
				log.Printf("Error downloading artifact: %v", err)
				continue
			}
			w.Header().Set("Content-Type", "text/html")
			w.Write(data)
			return
		}
	}

	http.Error(w, "No HTML report found", http.StatusNotFound)
}

func (s *Server) handleExecutionLogs(w http.ResponseWriter, r *http.Request) {
	// id := chi.URLParam(r, "id")
	// For now, return placeholder logs
	w.Write([]byte("Logs not yet implemented"))
}

func (s *Server) handleFlakyTestsAPI(w http.ResponseWriter, r *http.Request) {
	flakyTests, err := s.db.GetFlakyTests(0.1)
	if err != nil {
		log.Printf("Error getting flaky tests: %v", err)
		http.Error(w, "Failed to load flaky tests", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(flakyTests)
}

func (s *Server) render(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html")
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
