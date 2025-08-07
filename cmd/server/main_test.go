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

func TestHomeHandler_NoChurch(t *testing.T) {
	templateCache, err := newTemplateCache()
	require.NoError(t, err)
	app := &application{templates: templateCache}

	req, err := http.NewRequest("GET", "/", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(app.homeHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Welcome to Orthodox Pilgrimage")
}

func TestHomeHandler_WithChurch(t *testing.T) {
	dsn := "file::memory:?cache=shared&_busy_timeout=5000"
	dbConn, err := db.New(dsn)
	require.NoError(t, err)
	defer dbConn.Close()
	err = db.MigrateUp(dbConn)
	require.NoError(t, err)

	templateCache, err := newTemplateCache()
	require.NoError(t, err)

	app := &application{
		db:        sqlcdb.New(dbConn),
		templates: templateCache,
	}
	err = app.seedDatabase(context.Background())
	require.NoError(t, err)

	req, err := http.NewRequest("GET", "/churches/1", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(app.homeHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Welcome to Orthodox Pilgrimage")
}

func TestListChurchesHandler(t *testing.T) {
	dsn := "file::memory:?cache=shared&_busy_timeout=5000"
	dbConn, err := db.New(dsn)
	require.NoError(t, err)
	defer dbConn.Close()

	err = db.MigrateUp(dbConn)
	require.NoError(t, err)

	app := &application{
		db: sqlcdb.New(dbConn),
	}
	err = app.seedDatabase(context.Background())
	require.NoError(t, err)

	req, err := http.NewRequest("GET", "/api/v1/churches", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(app.listChurchesHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var churches []sqlcdb.Church
	err = json.Unmarshal(rr.Body.Bytes(), &churches)
	require.NoError(t, err)
	assert.Len(t, churches, 2)
}

func TestListChurchesHandler_Bounds(t *testing.T) {
	dsn := "file::memory:?cache=shared&_busy_timeout=5000"
	dbConn, err := db.New(dsn)
	require.NoError(t, err)
	defer dbConn.Close()

	err = db.MigrateUp(dbConn)
	require.NoError(t, err)

	app := &application{
		db: sqlcdb.New(dbConn),
	}
	err = app.seedDatabase(context.Background())
	require.NoError(t, err)

	// Bounding box that only includes the Chicago church
	url := "/api/v1/churches?minLat=41.8&maxLat=42.0&minLng=-88.0&maxLng=-87.0"
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(app.listChurchesHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var churches []sqlcdb.Church
	err = json.Unmarshal(rr.Body.Bytes(), &churches)
	require.NoError(t, err)
	assert.Len(t, churches, 1)
	assert.Equal(t, "Holy Trinity Orthodox Cathedral", churches[0].Name)
}

func TestChurchDetailHandler(t *testing.T) {
	dsn := "file::memory:?cache=shared&_busy_timeout=5000"
	dbConn, err := db.New(dsn)
	require.NoError(t, err)
	defer dbConn.Close()
	err = db.MigrateUp(dbConn)
	require.NoError(t, err)

	templateCache, err := newTemplateCache()
	require.NoError(t, err)

	app := &application{
		db:        sqlcdb.New(dbConn),
		templates: templateCache,
	}
	err = app.seedDatabase(context.Background())
	require.NoError(t, err)

	req, err := http.NewRequest("GET", "/churches/1", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(app.churchDetailHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "St. John the Baptist Greek Orthodox Church")
	assert.Equal(t, "/churches/1", rr.Header().Get("HX-Push-Url"))
}
