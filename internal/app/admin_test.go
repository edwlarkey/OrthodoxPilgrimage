package app

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/edwlarkey/orthodoxpilgrimage/internal/db"
	"github.com/edwlarkey/orthodoxpilgrimage/internal/db/sessionstore"
	sqlcdb "github.com/edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
	"github.com/edwlarkey/orthodoxpilgrimage/internal/ui"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAdminTest(t *testing.T) (*Application, *sql.DB) {
	t.Helper()
	dsn := "file::memory:?cache=shared"
	dbConn, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	err = db.MigrateUp(dbConn)
	require.NoError(t, err)

	tmplMgr, err := ui.NewTemplateManager()
	require.NoError(t, err)

	sessionManager := scs.New()
	sessionManager.Store = sessionstore.New(dbConn)
	sessionManager.Lifetime = 24 * time.Hour

	queries := sqlcdb.New(dbConn)
	appInstance := &Application{
		DB:             queries,
		DBConn:         dbConn,
		Templates:      tmplMgr,
		SessionManager: sessionManager,
	}
	return appInstance, dbConn
}

func TestAdminAuthFlow(t *testing.T) {
	app, dbConn := setupAdminTest(t)
	defer dbConn.Close()

	// 1. Initial login should redirect to setup since no admins exist
	req := httptest.NewRequest("GET", "/admin/login", nil)
	rr := httptest.NewRecorder()
	app.adminLoginHandler(rr, req)
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/admin/setup", rr.Header().Get("Location"))

	// 2. Perform setup
	form := url.Values{}
	form.Add("username", "testadmin")
	form.Add("password", "pass1234")
	req = httptest.NewRequest("POST", "/admin/setup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	app.adminSetupHandler(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Admin created!")

	// Extract MFA secret from response
	parts := strings.Split(rr.Body.String(), ": ")
	require.True(t, len(parts) > 1)
	mfaSecret := strings.Split(parts[1], "\n")[0]

	// 3. Login with correct credentials
	form = url.Values{}
	form.Add("username", "testadmin")
	form.Add("password", "pass1234")
	req = httptest.NewRequest("POST", "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(http.HandlerFunc(app.adminLoginHandler)).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/admin/mfa", rr.Header().Get("Location"))

	// Get the session cookie
	cookie := rr.Header().Get("Set-Cookie")
	assert.NotEmpty(t, cookie)

	// 4. Verify MFA
	code, err := totp.GenerateCode(mfaSecret, time.Now())
	require.NoError(t, err)

	form = url.Values{}
	form.Add("code", code)
	req = httptest.NewRequest("POST", "/admin/mfa", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", cookie)

	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(http.HandlerFunc(app.adminMfaHandler)).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/admin/dashboard", rr.Header().Get("Location"))
}

func TestAdminChurchEditData(t *testing.T) {
	app, dbConn := setupAdminTest(t)
	defer dbConn.Close()

	ctx := context.Background()
	// Create a church
	church, err := app.DB.CreateChurch(ctx, sqlcdb.CreateChurchParams{
		Name:          "Test Church",
		Slug:          "test-church",
		Type:          sql.NullString{String: "church", Valid: true},
		AddressText:   "123 Street",
		City:          "City",
		StateProvince: "ST",
		CountryCode:   "US",
		Latitude:      40.0,
		Longitude:     -70.0,
		Status:        "published",
	})
	require.NoError(t, err)

	// Create a saint
	saint, err := app.DB.CreateSaint(ctx, sqlcdb.CreateSaintParams{
		Name:   "St. Nicholas",
		Slug:   "st-nicholas",
		Status: "published",
	})
	require.NoError(t, err)

	// Create a relic
	err = app.DB.CreateRelic(ctx, sqlcdb.CreateRelicParams{
		ChurchID:    church.ID,
		SaintID:     saint.ID,
		Description: sql.NullString{String: "My relic", Valid: true},
	})
	require.NoError(t, err)

	// Mock session
	sessionHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), "username", "testadmin")
		app.SessionManager.Put(r.Context(), "admin_id", int64(1))
		app.adminChurchEditHandler(w, r)
	})

	req := httptest.NewRequest("GET", "/admin/churches/edit/test-church", nil)
	rr := httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	// We can't easily check the map[string]any passed to ExecuteTemplate from here
	// but we can check if the output contains the relic and saint names.
	body := rr.Body.String()
	assert.Contains(t, body, "St. Nicholas")
	assert.Contains(t, body, "My relic")
}

func TestAdminAuthMiddleware(t *testing.T) {
	app, dbConn := setupAdminTest(t)
	defer dbConn.Close()

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("protected content"))
	})
	handler := app.AdminAuthMiddleware(nextHandler)

	// 1. Unauthenticated request should redirect to login
	req := httptest.NewRequest("GET", "/admin/dashboard", nil)
	rr := httptest.NewRecorder()
	app.SessionManager.LoadAndSave(handler).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/admin/login", rr.Header().Get("Location"))

	// 2. MFA pending request should redirect to MFA
	setupSessionHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), "admin_id", int64(1))
		app.SessionManager.Put(r.Context(), "mfa_pending", true)
		w.WriteHeader(http.StatusOK)
	})

	rr = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/setup", nil)
	app.SessionManager.LoadAndSave(setupSessionHandler).ServeHTTP(rr, req)
	cookie := rr.Header().Get("Set-Cookie")

	// Now try to access dashboard with that cookie
	req = httptest.NewRequest("GET", "/admin/dashboard", nil)
	req.Header.Set("Cookie", cookie)
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(handler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/admin/mfa", rr.Header().Get("Location"))
}
