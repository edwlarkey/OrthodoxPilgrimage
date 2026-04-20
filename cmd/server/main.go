package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/edwlarkey/orthodoxpilgrimage/internal/app"
	internaldb "github.com/edwlarkey/orthodoxpilgrimage/internal/db"
	sqlcdb "github.com/edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
	"github.com/edwlarkey/orthodoxpilgrimage/internal/ui"
)

func main() {
	flag.Parse()

	dsn := "orthodox_pilgrimage.db?_busy_timeout=5000"
	dbConn, err := internaldb.New(dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer dbConn.Close()
	log.Println("Database connection successful")

	if err := internaldb.MigrateUp(dbConn); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("Database migrations successful")

	queries := sqlcdb.New(dbConn)

	// Seed database on every startup from data.json (source of truth)
	log.Println("Syncing database with data/data.json...")
	if err := app.SeedDatabase(context.Background(), queries); err != nil {
		log.Fatalf("failed to seed database: %v", err)
	}
	log.Println("Database synced successfully")

	log.Println("Generating sitemap.xml...")
	if err := app.GenerateSitemap(context.Background(), queries, "https://orthodoxpilgrimage.com"); err != nil {
		log.Fatalf("failed to generate sitemap: %v", err)
	}
	log.Println("Sitemap generated successfully")

	tmplMgr, err := ui.NewTemplateManager()
	if err != nil {
		log.Fatalf("failed to create template manager: %v", err)
	}

	application := &app.Application{
		DB:        queries,
		Templates: tmplMgr,
	}

	log.Println("Starting server on :8080")
	err = http.ListenAndServe(":8080", application.Routes())
	if err != nil {
		log.Fatal(err)
	}
}
