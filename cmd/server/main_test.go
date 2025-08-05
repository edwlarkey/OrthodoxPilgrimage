package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.sr.ht/~edwlarkey/orthodoxpilgrimage/internal/db"
	sqlcdb "git.sr.ht/~edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListChurchesHandler(t *testing.T) {
	// 1. Setup: Create an in-memory database, migrate it, and seed it.
	dsn := "file::memory:?cache=shared&_busy_timeout=5000"
	dbConn, err := db.New(dsn)
	require.NoError(t, err)
	defer dbConn.Close()

	err = db.MigrateUp(dbConn)
	require.NoError(t, err)

	// Create a new application instance with the test database.
	app := &application{
		db: sqlcdb.New(dbConn),
	}

	// Seed the database with a known church.
	err = app.seedDatabase(context.Background())
	require.NoError(t, err)

	// 2. Execution: Create a new HTTP request and recorder.
	req, err := http.NewRequest("GET", "/api/v1/churches", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(app.listChurchesHandler)

	// 3. Action: Serve the HTTP request.
	handler.ServeHTTP(rr, req)

	// 4. Assertion: Check the status code and response body.
	assert.Equal(t, http.StatusOK, rr.Code, "handler returned wrong status code")

	// Decode the JSON response.
	var churches []sqlcdb.Church
	err = json.Unmarshal(rr.Body.Bytes(), &churches)
	require.NoError(t, err, "failed to unmarshal response body")

	assert.Len(t, churches, 1, "expected to find one church")
	assert.Equal(t, "St. John the Baptist Greek Orthodox Church", churches[0].Name, "unexpected church name")
}
