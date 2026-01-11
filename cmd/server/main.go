package main

import (
	"log"
	"net/http"

	"github.com/testkube/dashboard/internal/database"
	"github.com/testkube/dashboard/internal/server"
	"github.com/testkube/dashboard/internal/testkube"
)

func main() {
	// Initialize Mock Clients
	api := testkube.NewMockClient()
	db := database.NewMockDatabase()

	srv := server.NewServer(api, db)

	port := ":8080"
	log.Printf("Starting Testkube Dashboard on %s", port)
	if err := http.ListenAndServe(port, srv.Router()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
