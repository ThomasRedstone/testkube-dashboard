package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/testkube/dashboard/internal/database"
	"github.com/testkube/dashboard/internal/testkube"
)

func TestHandleDashboard(t *testing.T) {
	// Create a mock Testkube client and database
	api := testkube.NewMockClient()
	db := database.NewMockDatabase()

	// Create a new server with the mock clients
	srv := NewServer(api, db, nil, "../..")

	// Create a new HTTP request
	req, err := http.NewRequest("GET", "/", nil)
	assert.NoError(t, err)

	// Create a new response recorder
	rr := httptest.NewRecorder()

	// Call the handler function
	srv.Router().ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusOK, rr.Code)

	// Check the response body
	assert.Contains(t, rr.Body.String(), "Testkube Dashboard")
}
