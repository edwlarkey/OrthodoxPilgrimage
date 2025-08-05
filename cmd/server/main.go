package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"log"
	"net/http"

	"git.sr.ht/~edwlarkey/orthodoxpilgrimage/internal/db"
	sqlcdb "git.sr.ht/~edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
)

// application holds the application-wide dependencies.
type application struct {
	db *sqlcdb.Queries
}

func main() {
	seed := flag.Bool("seed", false, "Seed the database with initial data")
	flag.Parse()

	// Connect to the database.
	dsn := "orthodox_pilgrimage.db?_busy_timeout=5000"
	dbConn, err := db.New(dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer dbConn.Close()
	log.Println("Database connection successful")

	// Run migrations.
	if err := db.MigrateUp(dbConn); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("Database migrations successful")

	// Create a new application instance.
	app := &application{
		db: sqlcdb.New(dbConn),
	}

	// If the -seed flag is provided, seed the database and exit.
	if *seed {
		log.Println("Seeding database...")
		if err := app.seedDatabase(context.Background()); err != nil {
			log.Fatalf("failed to seed database: %v", err)
		}
		log.Println("Database seeded successfully")
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", app.homeHandler)
	mux.HandleFunc("/api/v1/churches", app.listChurchesHandler)

	log.Println("Starting server on :8080")
	err = http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatal(err)
	}
}

// homeHandler is the handler for the root path.
// It explicitly checks for the "/" path to avoid catching all other routes.
func (app *application) homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Server is running!"))
}

// listChurchesHandler retrieves and returns a list of all churches.
func (app *application) listChurchesHandler(w http.ResponseWriter, r *http.Request) {
	churches, err := app.db.ListChurches(r.Context())
	if err != nil {
		http.Error(w, "Failed to retrieve churches", http.StatusInternalServerError)
		log.Printf("Error retrieving churches: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(churches); err != nil {
		http.Error(w, "Failed to encode churches to JSON", http.StatusInternalServerError)
		log.Printf("Error encoding churches: %v", err)
	}
}

func (app *application) seedDatabase(ctx context.Context) error {
	// For now, we are not implementing the geocoding logic.
	// We will add it later. We are hardcoding the coordinates for now.
	_, err := app.db.CreateChurch(ctx, sqlcdb.CreateChurchParams{
		Name:          "St. John the Baptist Greek Orthodox Church",
		AddressText:   "123 Main St, New York, NY 10001",
		City:          "New York",
		StateProvince: "NY",
		CountryCode:   "US",
		Latitude:      40.7128,
		Longitude:     -74.0060,
		Jurisdiction: sql.NullString{
			String: "Greek Orthodox Archdiocese of America",
			Valid:  true,
		},
	})
	return err
}
