package main

import (
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/testkube/dashboard/internal/app"
	"github.com/testkube/dashboard/internal/k8s"
)

type Server struct {
	k8sService app.K8sService
	router     *chi.Mux
	templates  map[string]*template.Template
	layout     *template.Template
}

func NewServer(k8sService app.K8sService) *Server {
	s := &Server{
		k8sService: k8sService,
		router:     chi.NewRouter(),
		templates:  make(map[string]*template.Template),
	}
	s.initTemplates()
	s.initRoutes()
	return s
}

func (s *Server) initRoutes() {
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)

	// Serve static files
	workDir := "." // Assuming running from root
	filesDir := http.Dir(filepath.Join(workDir, "web/static"))
	FileServer(s.router, "/static", filesDir)

	s.router.Get("/", s.handleIndex)
	s.router.Get("/tests", s.handleListTests)
	s.router.Get("/tests/{name}", s.handleTestDetails)
	s.router.Post("/tests/{name}/run", s.handleRunTest)
	s.router.Get("/tests/{name}/executions/{executionID}/logs", s.handleExecutionLogs)
}

func (s *Server) initTemplates() {
	var err error
	s.layout, err = template.ParseFiles("web/templates/layout.html")
	if err != nil {
		log.Fatalf("failed to parse layout: %v", err)
	}

	// Pre-parse pages that use the layout
	pages := []string{"index.html", "test_detail.html"}
	for _, page := range pages {
		t, err := s.layout.Clone()
		if err != nil {
			log.Fatalf("failed to clone layout for %s: %v", page, err)
		}
		_, err = t.ParseFiles(filepath.Join("web/templates", page))
		if err != nil {
			log.Fatalf("failed to parse %s: %v", page, err)
		}
		s.templates[page] = t
	}

	// Pre-parse fragments
	fragments := []string{"test_list.html"}
	for _, frag := range fragments {
		t, err := template.ParseFiles(filepath.Join("web/templates", frag))
		if err != nil {
			log.Fatalf("failed to parse fragment %s: %v", frag, err)
		}
		s.templates[frag] = t
	}
}

func (s *Server) render(w http.ResponseWriter, templateName string, data interface{}) {
	t, ok := s.templates[templateName]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	err := t.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.render(w, "index.html", nil)
}

func (s *Server) handleListTests(w http.ResponseWriter, r *http.Request) {
	tests, err := s.k8sService.ListTests(r.Context(), "testkube") // Hardcoded namespace for now
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		t, ok := s.templates["test_list.html"]
		if !ok {
			http.Error(w, "Template test_list.html not found", http.StatusInternalServerError)
			return
		}
		err := t.Execute(w, tests)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (s *Server) handleTestDetails(w http.ResponseWriter, r *http.Request) {
	testName := chi.URLParam(r, "name")
	namespace := "testkube" // Hardcoded for now

	test, err := s.k8sService.GetTest(r.Context(), namespace, testName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	executions, err := s.k8sService.ListExecutions(r.Context(), namespace, testName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Test       *app.Test
		Executions []app.TestExecution
	}{
		Test:       test,
		Executions: executions,
	}

	s.render(w, "test_detail.html", data)
}

func (s *Server) handleRunTest(w http.ResponseWriter, r *http.Request) {
	// Mock run
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Started"))
}

func (s *Server) handleExecutionLogs(w http.ResponseWriter, r *http.Request) {
	executionID := chi.URLParam(r, "executionID")
	namespace := "testkube"

	logs, err := s.k8sService.GetExecutionLogs(r.Context(), namespace, executionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(logs))
}

func main() {
	// In the future, we will toggle between Mock and Real based on config
	k8sService := k8s.NewMockK8sService()

	server := NewServer(k8sService)

	port := ":8080"
	log.Printf("Starting server on %s", port)
	if err := http.ListenAndServe(port, server.router); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// FileServer conveniently sets up a http.FileServer handler to serve
// static files from a http.FileSystem.
func FileServer(r chi.Router, path string, root http.FileSystem) {
	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
}
