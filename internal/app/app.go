package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	sqlcdb "github.com/edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
	"github.com/edwlarkey/orthodoxpilgrimage/internal/ui"
)

type Application struct {
	DB        *sqlcdb.Queries
	Templates *ui.TemplateManager
}

func (a *Application) SeedDatabase(ctx context.Context) error {
	return SeedDatabase(ctx, a.DB)
}

func (a *Application) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.homeHandler)
	mux.HandleFunc("/api/v1/churches", a.listChurchesHandler)
	mux.HandleFunc("/api/v1/search", a.searchHandler)
	mux.HandleFunc("/churches/", a.churchDetailHandler)
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "sitemap.xml")
	})
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		if strings.HasSuffix(r.Host, ".fly.dev") {
			fmt.Fprintln(w, "User-agent: *")
			fmt.Fprintln(w, "Disallow: /")
			return
		}
		fmt.Fprintln(w, "User-agent: *")
		fmt.Fprintln(w, "Allow: /")
		fmt.Fprintln(w, "Sitemap: https://orthodoxpilgrimage.com/sitemap.xml")
	})

	// Static files with caching headers for Cloudflare
	staticHandler := http.FileServer(http.FS(ui.StaticFS))
	mux.Handle("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=2592000, stale-while-revalidate=86400")
		staticHandler.ServeHTTP(w, r)
	}))

	return a.flyDevRobotsMiddleware(mux)
}

func (a *Application) flyDevRobotsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.Host, ".fly.dev") {
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		}
		next.ServeHTTP(w, r)
	})
}

type churchJSON struct {
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

type searchResult struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type ChurchWithRelics struct {
	Type    string
	Church  sqlcdb.Church
	Relics  []sqlcdb.ListRelicsForChurchRow
	Sources []string
}

type SaintWithType struct {
	Type            string
	Saint           sqlcdb.Saint
	ReferringChurch *sqlcdb.Church
	Churches        []sqlcdb.Church
}

func (a *Application) homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=86400, stale-while-revalidate=86400")
	w.Header().Set("Vary", "Accept-Encoding, HX-Request")
	var data any
	var err error

	if r.URL.Path != "/" {
		path := r.URL.Path
		if strings.HasPrefix(path, "/churches/") {
			slug := path[10:]
			church, churchErr := a.DB.GetChurchBySlug(r.Context(), slug)
			if churchErr == nil {
				relics, _ := a.DB.ListRelicsForChurch(r.Context(), church.ID)
				sources, _ := a.DB.ListSourcesForChurch(r.Context(), church.ID)
				data = ChurchWithRelics{
					Type:    "church",
					Church:  church,
					Relics:  relics,
					Sources: sources,
				}
			} else {
				err = churchErr
			}
		} else {
			// Check if it's a saint slug (paths like /st-seraphim-of-sarov)
			slug := strings.TrimPrefix(path, "/")
			saint, saintErr := a.DB.GetSaintBySlug(r.Context(), slug)
			if saintErr == nil {
				// Fetch all churches for this saint
				churches, _ := a.DB.ListChurchesBySaintSlug(r.Context(), slug)

				sData := SaintWithType{
					Type:     "saint",
					Saint:    saint,
					Churches: churches,
				}
				// Optional: link back to referring church
				fromSlug := r.URL.Query().Get("from")
				if fromSlug != "" {
					if refChurch, refErr := a.DB.GetChurchBySlug(r.Context(), fromSlug); refErr == nil {
						sData.ReferringChurch = &refChurch
					}
				}
				data = sData
			} else {
				err = saintErr
			}
		}

		if err != nil {
			if err == sql.ErrNoRows {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			log.Printf("Error fetching data for path %s: %v", path, err)
			return
		}
	}
	if err := a.Templates.Render(w, "index", data); err != nil {
		http.Error(w, "failed to render template", http.StatusInternalServerError)
		log.Printf("Error rendering template: %v", err)
	}
}

func (a *Application) listChurchesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	minLatStr := r.URL.Query().Get("minLat")
	maxLatStr := r.URL.Query().Get("maxLat")
	minLngStr := r.URL.Query().Get("minLng")
	maxLngStr := r.URL.Query().Get("maxLng")
	saintSlug := r.URL.Query().Get("saint")

	var churches []sqlcdb.Church
	var err error

	if saintSlug != "" {
		churches, err = a.DB.ListChurchesBySaintSlug(ctx, saintSlug)
	} else if minLatStr != "" && maxLatStr != "" && minLngStr != "" && maxLngStr != "" {
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
		churches, err = a.DB.ListChurchesInBounds(ctx, params)
	} else {
		churches, err = a.DB.ListChurches(ctx)
	}

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
			Slug:        c.Slug,
			AddressText: c.AddressText,
			City:        c.City,
			Latitude:    c.Latitude,
			Longitude:   c.Longitude,
			Website:     c.Website,
			Description: c.Description,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	if err := json.NewEncoder(w).Encode(churchesJSON); err != nil {
		http.Error(w, "Failed to encode churches to JSON", http.StatusInternalServerError)
		log.Printf("Error encoding churches: %v", err)
	}
}

func (a *Application) churchDetailHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	// Expecting /churches/{slug}
	if len(path) < 11 {
		http.NotFound(w, r)
		return
	}

	slug := path[10:]
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	church, err := a.DB.GetChurchBySlug(r.Context(), slug)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to retrieve church", http.StatusInternalServerError)
		log.Printf("Error retrieving church %s: %v", slug, err)
		return
	}

	relics, _ := a.DB.ListRelicsForChurch(r.Context(), church.ID)
	sources, _ := a.DB.ListSourcesForChurch(r.Context(), church.ID)
	data := ChurchWithRelics{
		Type:    "church",
		Church:  church,
		Relics:  relics,
		Sources: sources,
	}

	w.Header().Set("HX-Push-Url", fmt.Sprintf("/churches/%s", slug))
	w.Header().Set("Vary", "Accept-Encoding, HX-Request")

	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("Cache-Control", "public, max-age=86400, stale-while-revalidate=86400")
		ts, err := a.Templates.Get("church-detail")
		if err != nil {
			http.Error(w, "church-detail template not found", http.StatusInternalServerError)
			return
		}
		err = ts.ExecuteTemplate(w, "church-detail", data)
		if err != nil {
			http.Error(w, "failed to render church detail", http.StatusInternalServerError)
		}
	} else {
		w.Header().Set("Cache-Control", "public, max-age=86400, stale-while-revalidate=86400")
		if err := a.Templates.Render(w, "index", data); err != nil {
			http.Error(w, "failed to render template", http.StatusInternalServerError)
			log.Printf("Error rendering template: %v", err)
		}
	}
}

func (a *Application) HomeHandler(w http.ResponseWriter, r *http.Request) {
	a.homeHandler(w, r)
}

func (a *Application) ListChurchesHandler(w http.ResponseWriter, r *http.Request) {
	a.listChurchesHandler(w, r)
}

func (a *Application) ChurchDetailHandler(w http.ResponseWriter, r *http.Request) {
	a.churchDetailHandler(w, r)
}

func (a *Application) searchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if len(query) < 2 {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode([]searchResult{}); err != nil {
			http.Error(w, "Failed to encode search results", http.StatusInternalServerError)
			log.Printf("Error encoding search results: %v", err)
		}
		return
	}

	searchTerm := "%" + query + "%"
	saints, err := a.DB.SearchSaints(r.Context(), searchTerm)
	if err != nil {
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	results := make([]searchResult, len(saints))
	for i, s := range saints {
		results[i] = searchResult{
			Name: s.Name,
			Slug: s.Slug,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60, s-maxage=3600")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		http.Error(w, "Failed to encode search results", http.StatusInternalServerError)
		log.Printf("Error encoding search results: %v", err)
	}
}
