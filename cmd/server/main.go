package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/edwlarkey/orthodoxpilgrimage/internal/app"
	internaldb "github.com/edwlarkey/orthodoxpilgrimage/internal/db"
	"github.com/edwlarkey/orthodoxpilgrimage/internal/db/sessionstore"
	sqlcdb "github.com/edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
	"github.com/edwlarkey/orthodoxpilgrimage/internal/ui"
)

func main() {
	logFormat := flag.String("log-format", "text", "log format: text or json")
	devMode := flag.Bool("dev", false, "enable development mode")
	seed := flag.Bool("seed", false, "seed database from data/data.json")
	flag.Parse()

	var handler slog.Handler
	switch *logFormat {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, nil)
	default:
		handler = slog.NewTextHandler(os.Stdout, nil)
	}
	slog.SetDefault(slog.New(handler))

	dsn := "orthodox_pilgrimage.db?_busy_timeout=5000"
	dbConn, err := internaldb.New(dsn)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := dbConn.Close(); err != nil {
			slog.Error("failed to close database connection", "error", err)
		}
	}()
	slog.Info("Database connection successful")

	if err := internaldb.MigrateUp(dbConn); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	slog.Info("Database migrations successful")

	queries := sqlcdb.New(dbConn)

	// Initialize session manager
	sessionManager := scs.New()
	sessionManager.Store = sessionstore.New(dbConn)
	sessionManager.Lifetime = 24 * time.Hour
	sessionManager.Cookie.Persist = true
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode
	sessionManager.Cookie.Secure = !*devMode // Only secure in non-dev (production)

	// Seed database on every startup from data.json (source of truth)
	slog.Info("Syncing database with data/data.json...")
	if err := app.SeedDatabase(context.Background(), queries); err != nil {
		slog.Error("failed to seed database", "error", err)
		os.Exit(1)
	}
	slog.Info("Database synced successfully")

	slog.Info("Generating sitemap.xml...")
	if err := app.GenerateSitemap(context.Background(), queries, "https://orthodoxpilgrimage.com"); err != nil {
		slog.Error("failed to generate sitemap", "error", err)
		os.Exit(1)
	}
	slog.Info("Sitemap generated successfully")

	tmplMgr, err := ui.NewTemplateManager()
	if err != nil {
		slog.Error("failed to create template manager", "error", err)
		os.Exit(1)
	}

	application := &app.Application{
		DB:             queries,
		DBConn:         dbConn,
		Templates:      tmplMgr,
		SessionManager: sessionManager,
		DevMode:        *devMode,
	}

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      application.Routes(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	slog.Info("Starting server on :8080")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
