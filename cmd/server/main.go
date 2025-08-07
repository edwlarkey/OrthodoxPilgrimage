package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"

	"git.sr.ht/~edwlarkey/orthodoxpilgrimage/internal/db"
	sqlcdb "git.sr.ht/~edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
	"git.sr.ht/~edwlarkey/orthodoxpilgrimage/internal/ui"
	// Import embedded templates from project root
)

//go:generate sh -c "cd ../../ && go run github.com/sqlc-dev/sqlc/cmd/sqlc generate"

// application holds the application-wide dependencies.
type application struct {
	db        *sqlcdb.Queries
	templates map[string]*template.Template
}

// seedDatabase is now a method on *application for test compatibility.
func (app *application) seedDatabase(ctx context.Context) error {
	return seedDatabase(ctx, app.db)
}

// churchJSON is a struct for serializing church data to JSON with camelCase keys.
type churchJSON struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	AddressText string         `json:"addressText"`
	City        string         `json:"city"`
	Latitude    float64        `json:"latitude"`
	Longitude   float64        `json:"longitude"`
	Website     sql.NullString `json:"website"`
	Description sql.NullString `json:"description"`
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

	queries := sqlcdb.New(dbConn)

	// If the -seed flag is provided, seed the database.
	if *seed {
		count, err := queries.CountChurches(context.Background())
		if err != nil {
			log.Fatalf("failed to count churches: %v", err)
		}

		if count == 0 {
			log.Println("Seeding database...")
			if err := seedDatabase(context.Background(), queries); err != nil {
				log.Fatalf("failed to seed database: %v", err)
			}
			log.Println("Database seeded successfully")
		} else {
			log.Println("Database already seeded")
		}
	}

	// Parse and cache templates.
	templateCache, err := newTemplateCache()
	if err != nil {
		log.Fatalf("failed to create template cache: %v", err)
	}

	// Create a new application instance.
	app := &application{
		db:        queries,
		templates: templateCache,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", app.homeHandler)
	mux.HandleFunc("/api/v1/churches", app.listChurchesHandler)
	mux.HandleFunc("/churches/", app.churchDetailHandler)

	log.Println("Starting server on :8080")
	err = http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatal(err)
	}
}

// newTemplateCache creates a template cache as a map[string]*template.Template.
func newTemplateCache() (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}

	entries, err := ui.TemplatesFS.ReadDir("templates")
	if err != nil {
		return nil, err
	}

	var templateFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".html" {
			templateFiles = append(templateFiles, "templates/"+entry.Name())
		}
	}

	if len(templateFiles) == 0 {
		return nil, fmt.Errorf("no template files found in embedded FS")
	}

	// Parse all templates together for shared definitions
	ts, err := template.New("").ParseFS(ui.TemplatesFS, templateFiles...)
	if err != nil {
		return nil, err
	}

	for _, page := range templateFiles {
		name := filepath.Base(page)
		name = name[:len(name)-len(filepath.Ext(name))] // strip .html
		cache[name] = ts
	}

	return cache, nil
}

// homeHandler renders the main map page.
// It can also handle loading a specific church by ID if the path is /churches/{id}.
func (app *application) homeHandler(w http.ResponseWriter, r *http.Request) {
	var data interface{}
	var err error

	if r.URL.Path != "/" {
		var id int64
		if _, err = fmt.Sscanf(r.URL.Path, "/churches/%d", &id); err == nil {
			data, err = app.db.GetChurch(r.Context(), id)
			if err != nil {
				if err == sql.ErrNoRows {
					http.NotFound(w, r)
					return
				}
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
		} else {
			http.NotFound(w, r)
			return
		}
	}

	app.render(w, "index", data)
}

// render is a helper function for rendering templates.
func (app *application) render(w http.ResponseWriter, name string, data interface{}) {
	ts, ok := app.templates[name]
	if !ok {
		http.Error(w, fmt.Sprintf("The template %s does not exist", name), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Execute the template by its name ("index" or "church-detail")
	err := ts.ExecuteTemplate(w, name[:len(name)-len(filepath.Ext(name))], data)
	if err != nil {
		log.Printf("Error executing template %s: %v", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// listChurchesHandler retrieves and returns a list of all churches.
func (app *application) listChurchesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	minLatStr := r.URL.Query().Get("minLat")
	maxLatStr := r.URL.Query().Get("maxLat")
	minLngStr := r.URL.Query().Get("minLng")
	maxLngStr := r.URL.Query().Get("maxLng")

	if minLatStr != "" && maxLatStr != "" && minLngStr != "" && maxLngStr != "" {
		minLat, err1 := strconv.ParseFloat(minLatStr, 64)
		maxLat, err2 := strconv.ParseFloat(maxLatStr, 64)
		minLng, err3 := strconv.ParseFloat(minLngStr, 64)
		maxLng, err4 := strconv.ParseFloat(maxLngStr, 64)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			http.Error(w, "Invalid bounding box parameters", http.StatusBadRequest)
			return
		}
		params := sqlcdb.ListChurchesInBoundsParams{
			Latitude:    minLat,
			Latitude_2:  maxLat,
			Longitude:   minLng,
			Longitude_2: maxLng,
		}
		churches, err := app.db.ListChurchesInBounds(ctx, params)
		if err != nil {
			http.Error(w, "Failed to retrieve churches", http.StatusInternalServerError)
			log.Printf("Error retrieving churches in bounds: %v", err)
			return
		}
		churchesJSON := make([]churchJSON, len(churches))
		for i, c := range churches {
			churchesJSON[i] = churchJSON{
				ID:          c.ID,
				Name:        c.Name,
				AddressText: c.AddressText,
				City:        c.City,
				Latitude:    c.Latitude,
				Longitude:   c.Longitude,
				Website:     c.Website,
				Description: c.Description,
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(churchesJSON); err != nil {
			http.Error(w, "Failed to encode churches to JSON", http.StatusInternalServerError)
			log.Printf("Error encoding churches: %v", err)
		}
		return
	}

	// fallback: return all churches
	churches, err := app.db.ListChurches(ctx)
	if err != nil {
		http.Error(w, "Failed to retrieve churches", http.StatusInternalServerError)
		log.Printf("Error retrieving churches: %v", err)
		return
	}

	churchesJSON := make([]churchJSON, len(churches))
	for i, c := range churches {
		churchesJSON[i] = churchJSON{
			ID:          c.ID,
			Name:        c.Name,
			AddressText: c.AddressText,
			City:        c.City,
			Latitude:    c.Latitude,
			Longitude:   c.Longitude,
			Website:     c.Website,
			Description: c.Description,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(churchesJSON); err != nil {
		http.Error(w, "Failed to encode churches to JSON", http.StatusInternalServerError)
		log.Printf("Error encoding churches: %v", err)
	}
}

// churchDetailHandler renders an HTML fragment for a single church's details.
func (app *application) churchDetailHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if len(path) < 11 {
		http.NotFound(w, r)
		return
	}

	idStr := path[10:]
	if idStr == "" {
		http.NotFound(w, r)
		return
	}

	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		http.NotFound(w, r)
		return
	}

	church, err := app.db.GetChurch(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to retrieve church", http.StatusInternalServerError)
		log.Printf("Error retrieving church %d: %v", id, err)
		return
	}

	// Push the URL to the browser's history
	w.Header().Set("HX-Push-Url", fmt.Sprintf("/churches/%d", id))

	app.render(w, "church-detail", church)
}

func seedDatabase(ctx context.Context, queries *sqlcdb.Queries) error {
	churches := []sqlcdb.CreateChurchParams{
		{
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
		},
		{
			Name:          "Holy Trinity Orthodox Cathedral",
			AddressText:   "1121 N Leavitt St, Chicago, IL 60622",
			City:          "Chicago",
			StateProvince: "IL",
			CountryCode:   "US",
			Latitude:      41.9022,
			Longitude:     -87.6818,
			Jurisdiction: sql.NullString{
				String: "Orthodox Church in America",
				Valid:  true,
			},
		},
	}

	for _, church := range churches {
		_, err := queries.CreateChurch(ctx, church)
		if err != nil {
			return fmt.Errorf("failed to create church %s: %w", church.Name, err)
		}
	}
	return nil
}
