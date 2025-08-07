package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"git.sr.ht/~edwlarkey/orthodoxpilgrimage/internal/app"
	internaldb "git.sr.ht/~edwlarkey/orthodoxpilgrimage/internal/db"
	sqlcdb "git.sr.ht/~edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
	"git.sr.ht/~edwlarkey/orthodoxpilgrimage/internal/ui"
)

func main() {
	seed := flag.Bool("seed", false, "Seed the database with initial data")
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

	if *seed {
		count, err := queries.CountChurches(context.Background())
		if err != nil {
			log.Fatalf("failed to count churches: %v", err)
		}
		if count == 0 {
			log.Println("Seeding database...")
			if err := app.SeedDatabase(context.Background(), queries); err != nil {
				log.Fatalf("failed to seed database: %v", err)
			}
			log.Println("Database seeded successfully")
		} else {
			log.Println("Database already seeded")
		}
	}

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
