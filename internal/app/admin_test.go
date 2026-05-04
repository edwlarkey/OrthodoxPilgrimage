package app

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/edwlarkey/orthodoxpilgrimage/internal/db"
	"github.com/edwlarkey/orthodoxpilgrimage/internal/db/sessionstore"
	sqlcdb "github.com/edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
	"github.com/edwlarkey/orthodoxpilgrimage/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
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

	// 3. Login with correct credentials
	form = url.Values{}
	form.Add("username", "testadmin")
	form.Add("password", "pass1234")
	req = httptest.NewRequest("POST", "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(http.HandlerFunc(app.adminLoginHandler)).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/admin/dashboard", rr.Header().Get("Location"))

	// Get the session cookie
	cookie := rr.Header().Get("Set-Cookie")
	assert.NotEmpty(t, cookie)
}

func TestAdminAuthEdgeCases(t *testing.T) {
	app, dbConn := setupAdminTest(t)
	defer dbConn.Close()

	ctx := context.Background()
	// Create an admin
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), 12)
	_, err := app.DB.CreateAdmin(ctx, sqlcdb.CreateAdminParams{
		Username:     "admin",
		PasswordHash: string(hash),
	})
	require.NoError(t, err)

	// 1. Login with invalid username
	form := url.Values{}
	form.Add("username", "wronguser")
	form.Add("password", "password123")
	req := httptest.NewRequest("POST", "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.SessionManager.LoadAndSave(http.HandlerFunc(app.adminLoginHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid username or password")

	// 2. Login with invalid password
	form = url.Values{}
	form.Add("username", "admin")
	form.Add("password", "wrongpass")
	req = httptest.NewRequest("POST", "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(http.HandlerFunc(app.adminLoginHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid username or password")

	// 3. Setup when admin already exists
	req = httptest.NewRequest("GET", "/admin/setup", nil)
	rr = httptest.NewRecorder()
	app.adminSetupHandler(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)

	// 4. Logout
	sessionHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), "admin_id", int64(1))
		app.adminLogoutHandler(w, r)
	})
	req = httptest.NewRequest("GET", "/admin/logout", nil)
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/admin/login", rr.Header().Get("Location"))
}

func TestAdminDashboard(t *testing.T) {
	app, dbConn := setupAdminTest(t)
	defer dbConn.Close()

	ctx := context.Background()
	// Seed some data
	_, _ = app.DB.CreateChurch(ctx, sqlcdb.CreateChurchParams{Name: "D Church", Slug: "d-church", CountryCode: "US", Status: "published"})
	_, _ = app.DB.CreateSaint(ctx, sqlcdb.CreateSaintParams{Name: "D Saint", Slug: "d-saint", Status: "published"})

	sessionHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), "username", "testadmin")
		app.adminDashboardHandler(w, r)
	})

	req := httptest.NewRequest("GET", "/admin/dashboard", nil)
	rr := httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "Dashboard")
	assert.Contains(t, body, "testadmin")
	assert.Contains(t, body, "D Church")
	assert.Contains(t, body, "D Saint")
}

func TestAdminSaintsHandlers(t *testing.T) {
	app, dbConn := setupAdminTest(t)
	defer dbConn.Close()

	ctx := context.Background()
	_, _ = app.DB.CreateSaint(ctx, sqlcdb.CreateSaintParams{
		Name: "Saint One",
		Slug: "saint-one",
	})

	sessionHandler := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			app.SessionManager.Put(r.Context(), "admin_id", int64(1))
			app.SessionManager.Put(r.Context(), "username", "testadmin")
			h(w, r)
		}
	}

	// List
	req := httptest.NewRequest("GET", "/admin/saints", nil)
	rr := httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminSaintsListHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Saint One")

	// GET Edit New
	req = httptest.NewRequest("GET", "/admin/saints/edit", nil)
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminSaintEditHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "New Saint")

	// GET Edit Existing
	req = httptest.NewRequest("GET", "/admin/saints/edit/saint-one", nil)
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminSaintEditHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Edit Saint")
	assert.Contains(t, rr.Body.String(), "Saint One")

	// GET Edit Non-existent
	req = httptest.NewRequest("GET", "/admin/saints/edit/non-existent", nil)
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminSaintEditHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)

	// POST Edit New
	form := url.Values{}
	form.Add("name", "Saint Two")
	form.Add("slug", "saint-two")
	form.Add("status", "published")
	req = httptest.NewRequest("POST", "/admin/saints/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminSaintEditHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, "/admin/saints", rr.Header().Get("HX-Location"))

	// POST Edit New Validation Error
	form = url.Values{}
	form.Add("name", "")
	req = httptest.NewRequest("POST", "/admin/saints/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminSaintEditHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	// POST Edit Existing
	form = url.Values{}
	form.Add("name", "Saint One Updated")
	form.Add("slug", "saint-one")
	form.Add("status", "published")
	req = httptest.NewRequest("POST", "/admin/saints/edit/saint-one", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminSaintEditHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Delete
	req = httptest.NewRequest("POST", "/admin/saints/delete/saint-one", nil)
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminSaintDeleteHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/admin/saints", rr.Header().Get("Location"))

	// Delete with HX-Request
	req = httptest.NewRequest("POST", "/admin/saints/delete/saint-two", nil)
	req.Header.Set("HX-Request", "true")
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminSaintDeleteHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestAdminChurchesHandlers(t *testing.T) {
	app, dbConn := setupAdminTest(t)
	defer dbConn.Close()

	ctx := context.Background()
	_, _ = app.DB.CreateChurch(ctx, sqlcdb.CreateChurchParams{
		Name:          "Church One",
		Slug:          "church-one",
		AddressText:   "Addr",
		City:          "City",
		StateProvince: "ST",
		CountryCode:   "US",
		Status:        "published",
	})

	sessionHandler := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			app.SessionManager.Put(r.Context(), "admin_id", int64(1))
			app.SessionManager.Put(r.Context(), "username", "testadmin")
			h(w, r)
		}
	}

	// List
	req := httptest.NewRequest("GET", "/admin/churches", nil)
	rr := httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminChurchesListHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Church One")

	// GET Edit Existing
	req = httptest.NewRequest("GET", "/admin/churches/edit/church-one", nil)
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminChurchEditHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Edit Church")
	assert.Contains(t, rr.Body.String(), "Church One")

	// POST Edit New
	form := url.Values{}
	form.Add("name", "Church Two")
	form.Add("slug", "church-two")
	form.Add("address_text", "Addr 2")
	form.Add("city", "City 2")
	form.Add("state_province", "ST 2")
	form.Add("country_code", "US")
	form.Add("status", "published")
	req = httptest.NewRequest("POST", "/admin/churches/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminChurchEditHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	// POST Edit Existing
	form = url.Values{}
	form.Add("name", "Church One Updated")
	form.Add("slug", "church-one")
	form.Add("address_text", "Addr")
	form.Add("city", "City")
	form.Add("state_province", "ST")
	form.Add("country_code", "US")
	form.Add("status", "published")
	req = httptest.NewRequest("POST", "/admin/churches/edit/church-one", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminChurchEditHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Delete
	req = httptest.NewRequest("POST", "/admin/churches/delete/church-one", nil)
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminChurchDeleteHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/admin/churches", rr.Header().Get("Location"))

	// Delete Non-existent
	req = httptest.NewRequest("POST", "/admin/churches/delete/non-existent", nil)
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminChurchDeleteHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
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
	// We check if the output contains the relic and saint names, ensuring data was loaded.
	body := rr.Body.String()
	assert.Contains(t, body, "St. Nicholas")
	assert.Contains(t, body, "My relic")
}

func TestAdminRelicsHandlers(t *testing.T) {
	app, dbConn := setupAdminTest(t)
	defer dbConn.Close()

	ctx := context.Background()
	church, _ := app.DB.CreateChurch(ctx, sqlcdb.CreateChurchParams{Name: "C", Slug: "c", CountryCode: "US", Status: "published"})
	saint, _ := app.DB.CreateSaint(ctx, sqlcdb.CreateSaintParams{Name: "S", Slug: "s", Status: "published"})
	saint2, _ := app.DB.CreateSaint(ctx, sqlcdb.CreateSaintParams{Name: "S2", Slug: "s2", Status: "published"})
	_ = app.DB.CreateRelic(ctx, sqlcdb.CreateRelicParams{ChurchID: church.ID, SaintID: saint.ID})

	sessionHandler := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			app.SessionManager.Put(r.Context(), "admin_id", int64(1))
			app.SessionManager.Put(r.Context(), "username", "testadmin")
			h(w, r)
		}
	}

	// List
	req := httptest.NewRequest("GET", "/admin/relics", nil)
	rr := httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminRelicsListHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// GET Edit
	req = httptest.NewRequest("GET", "/admin/relics/edit", nil)
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminRelicEditHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// POST Edit (Create)
	form := url.Values{}
	form.Add("church_id", strconv.FormatInt(church.ID, 10))
	form.Add("saint_id", strconv.FormatInt(saint2.ID, 10))
	form.Add("description", "Desc")
	req = httptest.NewRequest("POST", "/admin/relics/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminRelicEditHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Delete
	req = httptest.NewRequest("POST", "/admin/relics/delete?church_id="+strconv.FormatInt(church.ID, 10)+"&saint_id="+strconv.FormatInt(saint.ID, 10), nil)
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminRelicDeleteHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusSeeOther, rr.Code)
}

func TestAdminChurchSourceHandlers(t *testing.T) {
	app, dbConn := setupAdminTest(t)
	defer dbConn.Close()

	ctx := context.Background()
	church, _ := app.DB.CreateChurch(ctx, sqlcdb.CreateChurchParams{Name: "C", Slug: "c", CountryCode: "US", Status: "published"})

	sessionHandler := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			app.SessionManager.Put(r.Context(), "admin_id", int64(1))
			app.SessionManager.Put(r.Context(), "username", "testadmin")
			h(w, r)
		}
	}

	// Add Source
	form := url.Values{}
	form.Add("church_id", strconv.FormatInt(church.ID, 10))
	form.Add("source", "http://source.com")
	req := httptest.NewRequest("POST", "/admin/churches/sources/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminChurchSourceAddHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Add Source Validation Error
	form = url.Values{}
	form.Add("church_id", strconv.FormatInt(church.ID, 10))
	form.Add("source", "")
	req = httptest.NewRequest("POST", "/admin/churches/sources/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminChurchSourceAddHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	// Delete Source
	sources, _ := app.DB.ListSourcesForChurch(ctx, church.ID)
	require.NotEmpty(t, sources)
	req = httptest.NewRequest("POST", "/admin/churches/sources/delete?id="+strconv.FormatInt(sources[0].ID, 10), nil)
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminChurchSourceDeleteHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestAdminRelicImageHandlers(t *testing.T) {
	app, dbConn := setupAdminTest(t)
	defer dbConn.Close()

	ctx := context.Background()
	church, _ := app.DB.CreateChurch(ctx, sqlcdb.CreateChurchParams{Name: "C", Slug: "c", CountryCode: "US", Status: "published"})
	saint, _ := app.DB.CreateSaint(ctx, sqlcdb.CreateSaintParams{Name: "S", Slug: "s", Status: "published"})
	_ = app.DB.CreateRelic(ctx, sqlcdb.CreateRelicParams{ChurchID: church.ID, SaintID: saint.ID})

	sessionHandler := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			app.SessionManager.Put(r.Context(), "admin_id", int64(1))
			app.SessionManager.Put(r.Context(), "username", "testadmin")
			h(w, r)
		}
	}

	// Add Image
	form := url.Values{}
	form.Add("relic_church_id", strconv.FormatInt(church.ID, 10))
	form.Add("relic_saint_id", strconv.FormatInt(saint.ID, 10))
	form.Add("url", "http://image.com/img.jpg")
	form.Add("alt_text", "Alt")
	req := httptest.NewRequest("POST", "/admin/relics/images/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminRelicImageAddHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Delete Image
	images, _ := app.DB.ListImagesForRelic(ctx, sqlcdb.ListImagesForRelicParams{RelicChurchID: sql.NullInt64{Int64: church.ID, Valid: true}, RelicSaintID: sql.NullInt64{Int64: saint.ID, Valid: true}})
	require.NotEmpty(t, images)
	req = httptest.NewRequest("POST", "/admin/relics/images/delete?id="+strconv.FormatInt(images[0].ID, 10), nil)
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler(app.adminRelicImageDeleteHandler)).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
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

	// 2. Authenticated request should pass
	authSessionHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), "admin_id", int64(1))
		w.WriteHeader(http.StatusOK)
	})

	rr = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/setup", nil)
	app.SessionManager.LoadAndSave(authSessionHandler).ServeHTTP(rr, req)
	cookie := rr.Header().Get("Set-Cookie")

	// Now try to access dashboard with that cookie
	req = httptest.NewRequest("GET", "/admin/dashboard", nil)
	req.Header.Set("Cookie", cookie)
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(handler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAdminListAdminsHandler(t *testing.T) {
	app, dbConn := setupAdminTest(t)
	defer dbConn.Close()

	// Add an admin
	app.DB.CreateAdmin(context.Background(), sqlcdb.CreateAdminParams{
		Username:     "testadmin",
		PasswordHash: "hash",
	})

	sessionHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), "username", "testadmin")
		app.adminListAdminsHandler(w, r)
	})

	req := httptest.NewRequest("GET", "/admin/admins", nil)
	rr := httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "testadmin")
}

func TestAdminCreateAdminHandler(t *testing.T) {
	app, dbConn := setupAdminTest(t)
	defer dbConn.Close()

	sessionHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), "admin_id", int64(1))
		app.adminCreateAdminHandler(w, r)
	})

	// GET request should return form
	req := httptest.NewRequest("GET", "/admin/admins/new", nil)
	rr := httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Add New Admin")

	// POST request should create admin
	form := url.Values{}
	form.Add("username", "newadmin")
	form.Add("password", "securepassword123")
	req = httptest.NewRequest("POST", "/admin/admins/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Contains(t, rr.Header().Get("HX-Trigger"), "Admin created successfully")

	// Verify admin was created
	admins, err := app.DB.ListAdmins(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, len(admins))
	assert.Equal(t, "newadmin", admins[0].Username)

	// POST request with existing username should return error
	req = httptest.NewRequest("POST", "/admin/admins/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Username already exists")
}

func TestAdminDeleteAdmin(t *testing.T) {
	app, dbConn := setupAdminTest(t)
	defer dbConn.Close()

	// Create two admins: one active, one to delete
	activeAdmin, _ := app.DB.CreateAdmin(context.Background(), sqlcdb.CreateAdminParams{
		Username:     "active",
		PasswordHash: "hash",
	})
	toDeleteAdmin, _ := app.DB.CreateAdmin(context.Background(), sqlcdb.CreateAdminParams{
		Username:     "todelete",
		PasswordHash: "hash",
	})

	sessionHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), "admin_id", activeAdmin.ID)
		app.adminDeleteAdminHandler(w, r)
	})

	// 1. Try to delete self
	req := httptest.NewRequest("DELETE", "/admin/admins/delete?id="+strconv.FormatInt(activeAdmin.ID, 10), nil)
	rr := httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Contains(t, rr.Header().Get("HX-Trigger"), "You cannot delete your own account")

	// 2. Delete the other admin
	req = httptest.NewRequest("DELETE", "/admin/admins/delete?id="+strconv.FormatInt(toDeleteAdmin.ID, 10), nil)
	rr = httptest.NewRecorder()
	app.SessionManager.LoadAndSave(sessionHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("HX-Trigger"), "Admin deleted successfully")

	// Verify deletion
	admins, _ := app.DB.ListAdmins(context.Background())
	assert.Equal(t, 1, len(admins))
	assert.Equal(t, "active", admins[0].Username)
}
