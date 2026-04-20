package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/edwlarkey/orthodoxpilgrimage/internal/app"
	"github.com/edwlarkey/orthodoxpilgrimage/internal/db"
	sqlcdb "github.com/edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
	"github.com/edwlarkey/orthodoxpilgrimage/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDataJSON = `{
  "saints": [
    {
      "name": "Saint Seraphim of Sarov",
      "slug": "st-seraphim-of-sarov",
      "feast_day": "January 2",
      "description": "Russian monk",
      "lives_url": "https://example.com/st-seraphim",
      "updated_at": "2026-01-15"
    }
  ],
  "churches": [
    {
      "name": "St. John the Baptist",
      "slug": "st-john-baptist-ny",
      "address_text": "123 Main St",
      "city": "New York",
      "state_province": "NY",
      "country_code": "US",
      "latitude": 40.7128,
      "longitude": -74.0060,
      "relics": [
        {
          "saint_slug": "st-seraphim-of-sarov",
          "description": "small portion"
        }
      ],
      "sources": [
        "https://example.com/source1",
        "Called and confirmed"
      ],
      "updated_at": "2026-03-10"
    },
    {
      "name": "Holy Trinity",
      "slug": "holy-trinity-chicago",
      "address_text": "1121 N Leavitt St",
      "city": "Chicago",
      "state_province": "IL",
      "country_code": "US",
      "latitude": 41.9022,
      "longitude": -87.6818
    }
  ]
}`

type testChurchJSON struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	Slug        string         `json:"slug"`
	AddressText string         `json:"addressText"`
	City        string         `json:"city"`
	Latitude    float64        `json:"latitude"`
	Longitude   float64        `json:"longitude"`
	Website     sql.NullString `json:"website"`
	Description sql.NullString `json:"description"`
}

func seedTestDB(t *testing.T) (*app.Application, *sql.DB) {
	dsn := "file:memdb?mode=memory&cache=shared"
	dbConn, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	err = db.MigrateUp(dbConn)
	require.NoError(t, err)

	tmplMgr, err := ui.NewTemplateManager()
	require.NoError(t, err)

	queries := sqlcdb.New(dbConn)
	err = app.SeedFromReader(context.Background(), queries, strings.NewReader(testDataJSON))
	require.NoError(t, err)

	appInstance := &app.Application{
		DB:        queries,
		Templates: tmplMgr,
	}
	return appInstance, dbConn
}

func TestHomeHandler_NoChurch(t *testing.T) {
	tmplMgr, err := ui.NewTemplateManager()
	require.NoError(t, err)
	appInstance := &app.Application{Templates: tmplMgr, DB: nil}

	req, err := http.NewRequest("GET", "/", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(appInstance.HomeHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Welcome to Orthodox Pilgrimage")
}

func TestHomeHandler_WithChurch(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/churches/st-john-baptist-ny", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(appInstance.HomeHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "St. John the Baptist")
}

func TestHomeHandler_WithSaint(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	// Test viewing a saint directly
	req, err := http.NewRequest("GET", "/st-seraphim-of-sarov", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(appInstance.HomeHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Saint Seraphim of Sarov")
	assert.Contains(t, rr.Body.String(), "https://example.com/st-seraphim")
	assert.Contains(t, rr.Body.String(), "Read more about their life")

	// Test viewing a saint with a referring church
	req, err = http.NewRequest("GET", "/st-seraphim-of-sarov?from=st-john-baptist-ny", nil)
	require.NoError(t, err)

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Back to St. John the Baptist")
}

func TestListChurchesHandler(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/api/v1/churches", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(appInstance.ListChurchesHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var churches []testChurchJSON
	err = json.Unmarshal(rr.Body.Bytes(), &churches)
	require.NoError(t, err)
	assert.Len(t, churches, 2)
}

func TestListChurchesHandler_Saint(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/api/v1/churches?saint=st-seraphim-of-sarov", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(appInstance.ListChurchesHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var churches []testChurchJSON
	err = json.Unmarshal(rr.Body.Bytes(), &churches)
	require.NoError(t, err)
	assert.Len(t, churches, 1)
	assert.Equal(t, "St. John the Baptist", churches[0].Name)
}

func TestChurchDetailHandler(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/churches/st-john-baptist-ny", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(appInstance.ChurchDetailHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "St. John the Baptist")
	assert.Equal(t, "/churches/st-john-baptist-ny", rr.Header().Get("HX-Push-Url"))
}

func TestChurchDetailHandler_Sources(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/churches/st-john-baptist-ny", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(appInstance.ChurchDetailHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Sources")
	assert.Contains(t, rr.Body.String(), "https://example.com/source1")
	assert.Contains(t, rr.Body.String(), "Called and confirmed")
}

func TestChurchDetailHandler_NoSources(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/churches/holy-trinity-chicago", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(appInstance.ChurchDetailHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NotContains(t, rr.Body.String(), "Sources")
}

func TestChurchDetailHandler_SourceLinkRendering(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/churches/st-john-baptist-ny", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(appInstance.ChurchDetailHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	body := rr.Body.String()

	assert.Contains(t, body, `href="https://example.com/source1"`)
	assert.Contains(t, body, `target="_blank"`)
	assert.Contains(t, body, "Called and confirmed")
	assert.NotContains(t, body, `href="Called and confirmed"`)
}

func TestHomeHandler_ChurchWithSources(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/churches/st-john-baptist-ny", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(appInstance.HomeHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Sources")
	assert.Contains(t, rr.Body.String(), "https://example.com/source1")
}

func TestGenerateSitemap_UsesUpdatedDates(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	ctx := context.Background()

	err := app.GenerateSitemap(ctx, appInstance.DB, "https://example.com")
	require.NoError(t, err)

	data, err := os.ReadFile("sitemap.xml")
	require.NoError(t, err)
	defer os.Remove("sitemap.xml")

	body := string(data)

	assert.Contains(t, body, "<loc>https://example.com/churches/st-john-baptist-ny</loc>")
	assert.Contains(t, body, "<loc>https://example.com/st-seraphim-of-sarov</loc>")

	assert.Contains(t, body, "2026-03-10")
	assert.Contains(t, body, "2026-01-15")
}

func TestGenerateSitemap_NoUpdatedDate(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	ctx := context.Background()

	err := app.GenerateSitemap(ctx, appInstance.DB, "https://example.com")
	require.NoError(t, err)

	data, err := os.ReadFile("sitemap.xml")
	require.NoError(t, err)
	defer os.Remove("sitemap.xml")

	body := string(data)

	assert.Contains(t, body, "<loc>https://example.com/churches/holy-trinity-chicago</loc>")

	today := time.Now().Format("2006-01-02")
	assert.Contains(t, body, today)
}
