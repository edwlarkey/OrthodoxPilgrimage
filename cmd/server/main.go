package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/edwlarkey/orthodoxpilgrimage/internal/app"
	internaldb "github.com/edwlarkey/orthodoxpilgrimage/internal/db"
	"github.com/edwlarkey/orthodoxpilgrimage/internal/db/sessionstore"
	sqlcdb "github.com/edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
	"github.com/edwlarkey/orthodoxpilgrimage/internal/ui"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
)

func main() {
	logFormat := flag.String("log-format", "text", "log format: text or json")
	devMode := flag.Bool("dev", false, "enable development mode")
	seed := flag.Bool("seed", false, "seed database from data/data.json")
	dbPath := flag.String("db-path", "orthodox_pilgrimage.db", "path to the sqlite database file")
	flag.Parse()

	var handler slog.Handler
	switch *logFormat {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, nil)
	default:
		handler = slog.NewTextHandler(os.Stdout, nil)
	}
	slog.SetDefault(slog.New(handler))

	// S3 / Tigris Configuration
	imageBucket := os.Getenv("IMAGE_BUCKET")
	s3Endpoint := os.Getenv("AWS_ENDPOINT_URL_S3") // For Tigris: https://fly.storage.tigris.dev
	s3Region := os.Getenv("AWS_REGION")
	if s3Region == "" {
		s3Region = "auto"
	}

	var s3Client *s3.Client
	if imageBucket != "" && s3Endpoint != "" {
		cfg, err := config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(s3Region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				os.Getenv("AWS_ACCESS_KEY_ID"),
				os.Getenv("AWS_SECRET_ACCESS_KEY"),
				"",
			)),
		)
		if err != nil {
			slog.Error("failed to load S3 config", "error", err)
		} else {
			s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
				o.BaseEndpoint = aws.String(s3Endpoint)
			})
			slog.Info("S3 client initialized", "bucket", imageBucket, "endpoint", s3Endpoint)
		}
	} else {
		slog.Warn("S3 configuration missing; image uploads will be disabled", "bucket", imageBucket, "endpoint", s3Endpoint)
	}

	dsn := *dbPath + "?_busy_timeout=5000"
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

	// Seed database if requested
	if *seed {
		slog.Info("Syncing database with data/data.json...")
		if err := app.SeedDatabase(context.Background(), queries); err != nil {
			slog.Error("failed to seed database", "error", err)
			os.Exit(1)
		}
		slog.Info("Database synced successfully")
	}

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

	m := minify.New()
	m.AddFunc("text/css", css.Minify)

	application := &app.Application{
		DB:             queries,
		DBConn:         dbConn,
		Templates:      tmplMgr,
		SessionManager: sessionManager,
		S3Client:       s3Client,
		S3Bucket:       imageBucket,
		DevMode:        *devMode,
		Minifier:       m,
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
