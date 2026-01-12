package main

import (
	"log"
	"net/http"
	"os"

	"github.com/testkube/dashboard/internal/database"
	"github.com/testkube/dashboard/internal/server"
	"github.com/testkube/dashboard/internal/testkube"
	"github.com/testkube/dashboard/internal/users"
)

func main() {
	// Determine which client to use
	var api testkube.Client
	var err error

	useMock := os.Getenv("USE_MOCK") == "true"

	if useMock {
		log.Println("Using MOCK Testkube API client (USE_MOCK=true)")
		api = testkube.NewMockClient()
	} else {
		log.Println("Using REAL Testkube API client")
		apiURL := os.Getenv("TESTKUBE_API_URL")
		if apiURL == "" {
			apiURL = "http://testkube-api-server:8088"
		}
		log.Printf("Connecting to Testkube API: %s", apiURL)

		api, err = testkube.NewRealClient()
		if err != nil {
			log.Fatalf("Failed to create Testkube API client: %v", err)
		}
		log.Println("✓ Connected to Testkube API")
	}

	// Database still uses mock for Phase 2 (PostgreSQL comes in Phase 3)
	db := database.NewMockDatabase()

	var userGen *users.UserGenerator
	if os.Getenv("DATABASE_URL") != "" {
		var err error
		userGen, err = users.NewUserGenerator()
		if err != nil {
			log.Printf("Warning: User generator not available: %v", err)
		}
	}

	rootDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}

	srv := server.NewServer(api, db, userGen, rootDir)

	port := ":8080"
	log.Printf("Starting Testkube Dashboard on %s", port)
	if err := http.ListenAndServe(port, srv.Router()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
