package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/testkube/dashboard/internal/database"
	"github.com/testkube/dashboard/internal/environments"
	"github.com/testkube/dashboard/internal/testkube"
	"github.com/testkube/dashboard/internal/users"
)

type Server struct {
	api       testkube.Client
	db        database.Database
	envMgr    *environments.Manager
	userGen   *users.UserGenerator
	templates map[string]*template.Template
	rootDir   string
}

func NewServer(api testkube.Client, db database.Database, userGen *users.UserGenerator, rootDir string) *Server {
	// Load templates - each page needs its own template that includes layout
	templatesDir := filepath.Join(rootDir, "web/templates")
	templates := make(map[string]*template.Template)

	// List of page templates (each defines "content")
	pages := []string{
		"dashboard.html",
		"workflow_list.html",
		"workflow_detail.html",
		"execution_detail.html",
		"environments.html",
		"user_generator.html",
		"k6_report.html",
		"workflow_history.html",
	}

	layoutPath := filepath.Join(templatesDir, "layout.html")
	for _, page := range pages {
		pagePath := filepath.Join(templatesDir, page)
		// Parse layout first, then the page template
		t := template.Must(template.ParseFiles(layoutPath, pagePath))
		templates[page] = t
	}

	return &Server{
		api:       api,
		db:        db,
		envMgr:    environments.NewManager(),
		userGen:   userGen,
		templates: templates,
		rootDir:   rootDir,
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	// Static files
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(s.rootDir, "web/static")))))

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

	// Environment routes (UI)
	r.Get("/environments", s.handleEnvironmentList)
	r.Get("/environments/{id}", s.handleEnvironmentDetail)

	// Environment API routes
	r.Get("/api/v1/environments", s.handleEnvironmentsAPI)
	r.Post("/api/v1/environments", s.handleCreateEnvironmentAPI)
	r.Get("/api/v1/environments/{id}", s.handleGetEnvironmentAPI)
	r.Delete("/api/v1/environments/{id}", s.handleDeleteEnvironmentAPI)
	r.Post("/api/v1/environments/{id}/extend", s.handleExtendEnvironmentAPI)

	// Tools routes
	r.Get("/tools/user-generator", s.handleUserGeneratorPage)
	r.Get("/api/v1/users", s.handleListUsersAPI)
	r.Post("/api/v1/users", s.handleCreateUserAPI)
	r.Delete("/api/v1/users/{username}", s.handleDeleteUserAPI)
	r.Get("/api/v1/user-environments", s.handleListUserEnvironmentsAPI)

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
		"Error":          nil,
	}

	if trends != nil {
		data["PassRate"] = int(trends.CurrentPassRate * 100)
		data["PassRateTrend"] = trends.PassRateChange
		data["AvgDuration"] = trends.AvgDuration.String()
		data["DurationTrend"] = trends.DurationChange
	} else if err != nil {
		data["Error"] = fmt.Sprintf("Could not load trend data: %v", err)
	}

	s.render(w, "dashboard.html", data)
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

	s.render(w, "workflow_list.html", data)
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

	s.render(w, "workflow_detail.html", data)
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

	log.Printf("Found %d executions for workflow %s", len(executions), name)

	data := map[string]interface{}{
		"Name":       name,
		"Executions": executions,
	}

	s.render(w, "workflow_history.html", data)
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

	s.render(w, "execution_detail.html", data)
}

func (s *Server) handleExecutionReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	artifacts, err := s.api.GetArtifacts(id)
	if err != nil {
		log.Printf("Error getting artifacts: %v", err)
		http.Error(w, "Failed to load report", http.StatusInternalServerError)
		return
	}

	// Look for HTML report, prefer playwright
	var reportPath string
	for _, artifact := range artifacts {
		if artifact.Name == "playwright-report/index.html" {
			reportPath = artifact.Path
			break
		}
		if filepath.Ext(artifact.Name) == ".html" {
			reportPath = artifact.Path
		}
	}

	if reportPath != "" {
		data, err := s.api.DownloadArtifact(id, reportPath)
		if err != nil {
			log.Printf("Error downloading artifact %s: %v", reportPath, err)
			http.Error(w, "Failed to download report", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
		return
	}

	http.Error(w, "No HTML report found", http.StatusNotFound)
}

func (s *Server) handleExecutionLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	logs, err := s.api.GetExecutionLogs(id)
	if err != nil {
		log.Printf("Error getting execution logs: %v", err)
		http.Error(w, "Failed to load logs", http.StatusInternalServerError)
		return
	}
	w.Write([]byte(logs))
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

func (s *Server) render(w http.ResponseWriter, page string, data interface{}) {
	t, ok := s.templates[page]
	if !ok {
		log.Printf("Template not found: %s", page)
		http.Error(w, "Page not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Environment handlers

func (s *Server) handleEnvironmentList(w http.ResponseWriter, r *http.Request) {
	envs := s.envMgr.List(environments.ListEnvironmentsOptions{})

	data := map[string]interface{}{
		"Environments": envs,
		"Page":         "environments",
	}

	s.render(w, "environments.html", data)
}

func (s *Server) handleEnvironmentDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	env, err := s.envMgr.Get(id)
	if err != nil {
		http.Error(w, "Environment not found", http.StatusNotFound)
		return
	}

	// Calculate time remaining
	timeRemaining := time.Until(env.ExpiresAt)

	data := map[string]interface{}{
		"Environment":   env,
		"TimeRemaining": formatDuration(timeRemaining),
		"Page":          "environments",
	}

	s.render(w, "environments.html", data)
}

func (s *Server) handleEnvironmentsAPI(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	envs := s.envMgr.List(environments.ListEnvironmentsOptions{
		Owner: owner,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(envs)
}

func (s *Server) handleCreateEnvironmentAPI(w http.ResponseWriter, r *http.Request) {
	var req environments.CreateEnvironmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.Type == "" {
		req.Type = environments.TypeEphemeral
	}
	if req.Owner == "" {
		req.Owner = "anonymous"
	}

	env, err := s.envMgr.Create(r.Context(), req)
	if err != nil {
		log.Printf("Failed to create environment: %v", err)
		http.Error(w, "Failed to create environment", http.StatusInternalServerError)
		return
	}

	log.Printf("Created environment %s for %s", env.Name, env.Owner)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(env)
}

func (s *Server) handleGetEnvironmentAPI(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	env, err := s.envMgr.Get(id)
	if err != nil {
		http.Error(w, "Environment not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(env)
}

func (s *Server) handleDeleteEnvironmentAPI(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := s.envMgr.Delete(id); err != nil {
		http.Error(w, "Environment not found", http.StatusNotFound)
		return
	}

	log.Printf("Deleted environment %s", id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleExtendEnvironmentAPI(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Hours int `json:"hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Hours = 4 // Default extension
	}

	if err := s.envMgr.Extend(id, req.Hours); err != nil {
		http.Error(w, "Environment not found", http.StatusNotFound)
		return
	}

	env, _ := s.envMgr.Get(id)
	log.Printf("Extended environment %s by %d hours", id, req.Hours)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(env)
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return "Expired"
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// User Generator handlers

func (s *Server) handleUserGeneratorPage(w http.ResponseWriter, r *http.Request) {
	env := r.URL.Query().Get("env")
	if env == "" {
		env = os.Getenv("DATABASE_DEFAULT_SCHEMA")
	}

	var recentUsers []users.GeneratedUser
	var environments []users.Environment
	if s.userGen != nil {
		var err error
		environments, err = s.userGen.ListEnvironments()
		if err != nil {
			log.Printf("Error listing environments: %v", err)
		}
		recentUsers, err = s.userGen.ListRecentUsers(20, env)
		if err != nil {
			log.Printf("Error listing users: %v", err)
		}
		log.Printf("User Generator: %d environments, %d users in %s", len(environments), len(recentUsers), env)
	} else {
		log.Printf("User Generator: not available (userGen is nil)")
	}

	data := map[string]interface{}{
		"Page":            "tools",
		"RecentUsers":     recentUsers,
		"Environments":    environments,
		"CurrentEnv":      env,
		"DBAvailable":     s.userGen != nil,
	}

	s.render(w, "user_generator.html", data)
}

func (s *Server) handleListUsersAPI(w http.ResponseWriter, r *http.Request) {
	if s.userGen == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	env := r.URL.Query().Get("env")
	userList, err := s.userGen.ListRecentUsers(50, env)
	if err != nil {
		log.Printf("Error listing users: %v", err)
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userList)
}

func (s *Server) handleListUserEnvironmentsAPI(w http.ResponseWriter, r *http.Request) {
	if s.userGen == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	envs, err := s.userGen.ListEnvironments()
	if err != nil {
		log.Printf("Error listing environments: %v", err)
		http.Error(w, "Failed to list environments", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(envs)
}

func (s *Server) handleCreateUserAPI(w http.ResponseWriter, r *http.Request) {
	if s.userGen == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	var req users.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := s.userGen.CreateUser(req)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create user: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Created user: %s (%s) in %s", user.Username, user.Email, user.Environment)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

func (s *Server) handleDeleteUserAPI(w http.ResponseWriter, r *http.Request) {
	if s.userGen == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	username := chi.URLParam(r, "username")
	env := r.URL.Query().Get("env")
	if err := s.userGen.DeleteUser(username, env); err != nil {
		log.Printf("Error deleting user: %v", err)
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	log.Printf("Deleted user: %s", username)
	w.WriteHeader(http.StatusNoContent)
}
