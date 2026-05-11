package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/edwlarkey/orthodoxpilgrimage/internal/db"
	"github.com/edwlarkey/orthodoxpilgrimage/internal/db/sessionstore"
	sqlcdb "github.com/edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
	"github.com/edwlarkey/orthodoxpilgrimage/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
)

const testSeedData = `{
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
      "postal_code": "10001",
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
      "postal_code": "60622",
      "country_code": "US",
      "latitude": 41.9022,
      "longitude": -87.6818
    }
  ]
}`

func seedTestDB(t *testing.T) (*Application, *sql.DB) {
	t.Helper()
	dsn := "file:memdb?mode=memory&cache=shared"
	dbConn, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	err = db.MigrateUp(dbConn)
	require.NoError(t, err)

	tmplMgr, err := ui.NewTemplateManager()
	require.NoError(t, err)

	sessionManager := scs.New()
	sessionManager.Store = sessionstore.New(dbConn)

	queries := sqlcdb.New(dbConn)
	err = SeedFromReader(context.Background(), queries, strings.NewReader(testSeedData))
	require.NoError(t, err)

	m := minify.New()
	m.AddFunc("text/css", css.Minify)

	appInstance := &Application{
		DB:             queries,
		DBConn:         dbConn,
		Templates:      tmplMgr,
		SessionManager: sessionManager,
		Minifier:       m,
	}
	return appInstance, dbConn
}

// --- searchHandler tests ---

func TestSearchHandler_ShortQuery(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/api/v1/search?q=a", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	appInstance.searchHandler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var result []string
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestSearchHandler_ValidQuery(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/api/v1/search?q=Seraphim", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	appInstance.searchHandler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "public, max-age=60, s-maxage=3600", rr.Header().Get("Cache-Control"))
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var results []searchResult
	err = json.Unmarshal(rr.Body.Bytes(), &results)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Saint Seraphim of Sarov", results[0].Name)
	assert.Equal(t, "st-seraphim-of-sarov", results[0].Slug)
}

func TestSearchHandler_NoResults(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/api/v1/search?q=nonexistent", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	appInstance.searchHandler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var results []searchResult
	err = json.Unmarshal(rr.Body.Bytes(), &results)
	require.NoError(t, err)
	assert.Empty(t, results)
}

// --- Routes() tests ---

func TestRoutes_StaticFileCacheControl(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/static/style.css", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	appInstance.Routes().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "public, max-age=2592000, stale-while-revalidate=86400", rr.Header().Get("Cache-Control"))
	assert.Contains(t, rr.Body.String(), "body")
}

func TestRoutes_StaticFileNotFound(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/static/does-not-exist.css", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	appInstance.Routes().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// --- SeedFromReader error path tests ---

func TestSeedFromReader_InvalidJSON(t *testing.T) {
	dsn := "file:memdb?mode=memory&cache=shared"
	dbConn, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	defer dbConn.Close()

	require.NoError(t, db.MigrateUp(dbConn))
	queries := sqlcdb.New(dbConn)

	err = SeedFromReader(context.Background(), queries, strings.NewReader("{invalid json"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse data")
}

func TestSeedFromReader_UnknownSaintSlug(t *testing.T) {
	dsn := "file:memdb?mode=memory&cache=shared"
	dbConn, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	defer dbConn.Close()

	require.NoError(t, db.MigrateUp(dbConn))
	queries := sqlcdb.New(dbConn)

	badData := `{
		"saints": [],
		"churches": [
			{
				"name": "Test Church",
				"slug": "test-church",
				"latitude": 40.0,
				"longitude": -74.0,
				"relics": [
					{"saint_slug": "nonexistent-saint", "description": "test"}
				]
			}
		]
	}`

	err = SeedFromReader(context.Background(), queries, strings.NewReader(badData))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown saint slug")
}

// --- homeHandler error path tests ---

func TestHomeHandler_ChurchNotFound(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/churches/nonexistent-church", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	appInstance.HomeHandler(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHomeHandler_SaintNotFound(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/st-nonexistent-saint", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	appInstance.HomeHandler(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHomeHandler_SaintWithReferringChurch(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/st-seraphim-of-sarov?from=holy-trinity-chicago", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	appInstance.HomeHandler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Saint Seraphim of Sarov")
	assert.Contains(t, rr.Body.String(), "Back to Holy Trinity")
}

func TestHomeHandler_NilDBHome(t *testing.T) {
	tmplMgr, err := ui.NewTemplateManager()
	require.NoError(t, err)
	appInstance := &Application{Templates: tmplMgr, DB: nil}

	req, err := http.NewRequest("GET", "/", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	appInstance.HomeHandler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Select a location on the map")
}

// --- churchDetailHandler error path tests ---

func TestChurchDetailHandler_InvalidSlugTooShort(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/churches/x", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	appInstance.ChurchDetailHandler(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestChurchDetailHandler_NotFound(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/churches/nonexistent-church-xyz", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	appInstance.ChurchDetailHandler(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestChurchDetailHandler_HTMXRequest(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/churches/st-john-baptist-ny", nil)
	require.NoError(t, err)
	req.Header.Set("HX-Request", "true")

	rr := httptest.NewRecorder()
	appInstance.ChurchDetailHandler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "/churches/st-john-baptist-ny", rr.Header().Get("HX-Push-Url"))
	assert.Contains(t, rr.Body.String(), "St. John the Baptist")
	// Should NOT contain the full HTML (only the partial)
	assert.NotContains(t, rr.Body.String(), "<!DOCTYPE html>")
	assert.NotContains(t, rr.Body.String(), "<html")
}

// --- listChurchesHandler error path tests ---

func TestListChurchesHandler_InvalidBoundingBox(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	req, err := http.NewRequest("GET", "/api/v1/churches?minLat=abc&maxLat=1&minLng=1&maxLng=1", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	appInstance.ListChurchesHandler(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid bounding box parameters")
}

func TestListChurchesHandler_MissingBoundingBoxParams(t *testing.T) {
	appInstance, dbConn := seedTestDB(t)
	defer dbConn.Close()

	// Only some params provided — should fall through to ListChurches (all churches)
	req, err := http.NewRequest("GET", "/api/v1/churches?minLat=1&maxLat=1", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	appInstance.ListChurchesHandler(rr, req)

	// Should return all churches since not all bbox params are provided
	assert.Equal(t, http.StatusOK, rr.Code)
}

// --- TemplateManager tests ---

func TestTemplateManager_New(t *testing.T) {
	tm, err := ui.NewTemplateManager()
	require.NoError(t, err)
	assert.NotNil(t, tm)
}

func TestTemplateManager_Render(t *testing.T) {
	tmplMgr, err := ui.NewTemplateManager()
	require.NoError(t, err)

	var buf strings.Builder
	err = tmplMgr.Render(&buf, "base", nil)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "<!DOCTYPE html>")
	assert.Contains(t, buf.String(), "Orthodox Pilgrimage")
}

func TestTemplateManager_Render_NonExistentTemplate(t *testing.T) {
	tmplMgr, err := ui.NewTemplateManager()
	require.NoError(t, err)

	var buf strings.Builder
	// Render always looks up "base" from cache regardless of the name param
	// The cache has "base" so this succeeds; verify it renders correctly
	err = tmplMgr.Render(&buf, "anything", nil)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "<!DOCTYPE html>")
}

func TestTemplateManager_Get(t *testing.T) {
	tmplMgr, err := ui.NewTemplateManager()
	require.NoError(t, err)

	ts, err := tmplMgr.Get("base")
	require.NoError(t, err)
	assert.NotNil(t, ts)
}

func TestTemplateManager_Get_NotFound(t *testing.T) {
	tmplMgr, err := ui.NewTemplateManager()
	require.NoError(t, err)

	ts, err := tmplMgr.Get("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, ts)
	assert.Contains(t, err.Error(), "template nonexistent not found")
}

// --- db.New tests ---

func TestDBNew_InMemory(t *testing.T) {
	dsn := "file:memdb?mode=memory&cache=shared"
	dbConn, err := db.New(dsn)
	require.NoError(t, err)
	require.NotNil(t, dbConn)
	defer dbConn.Close()

	err = dbConn.Ping()
	assert.NoError(t, err)
}

func TestDBNew_InvalidDSN(t *testing.T) {
	_, err := db.New("file:/:nope/test.db")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to database")
}
