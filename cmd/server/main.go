package main

import (
	"log"
	"net/http"
	"os"

	"context"

	"github.com/testkube/dashboard/internal/database"
	"github.com/testkube/dashboard/internal/server"
	"github.com/testkube/dashboard/internal/testkube"
	"github.com/testkube/dashboard/internal/users"
	"github.com/testkube/dashboard/internal/worker"
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

	// Database configuration
	var db database.Database
	dbURL := os.Getenv("DATABASE_URL")

	if dbURL != "" {
		log.Println("Initializing PostgreSQL database...")
		pgDB, err := database.NewPostgresDatabase(dbURL)
		if err != nil {
			log.Printf("Failed to connect to PostgreSQL (falling back to mock): %v", err)
			db = database.NewMockDatabase()
		} else {
			log.Println("✓ Connected to PostgreSQL")
			db = pgDB
		}
	} else {
		log.Println("Using Mock Database (DATABASE_URL not set)")
		db = database.NewMockDatabase()
	}

	var userGen *users.UserGenerator
	if dbURL != "" {
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
	// Start background worker
	w := worker.NewWorker(api, db)
	// Run in a separate goroutine
	go w.Start(context.Background())

	if err := http.ListenAndServe(port, srv.Router()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
