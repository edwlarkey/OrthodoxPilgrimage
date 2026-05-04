package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
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
	mux.HandleFunc("/churches/", a.churchDetailHandler)
	mux.HandleFunc("/saints/", a.saintsDirectoryHandler)
	mux.HandleFunc("/api/v1/churches", a.listChurchesHandler)
	mux.HandleFunc("/api/v1/search", a.searchHandler)
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "sitemap.xml")
	})
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=2592000")
		r.URL.Path = "/static/favicon.ico"
		http.FileServer(http.FS(ui.StaticFS)).ServeHTTP(w, r)
	})
	mux.HandleFunc("/apple-touch-icon.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=2592000")
		r.URL.Path = "/static/apple-touch-icon.png"
		http.FileServer(http.FS(ui.StaticFS)).ServeHTTP(w, r)
	})
	mux.HandleFunc("/site.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json")
		w.Header().Set("Cache-Control", "public, max-age=2592000")
		r.URL.Path = "/static/site.webmanifest"
		http.FileServer(http.FS(ui.StaticFS)).ServeHTTP(w, r)
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

	return a.LoggingMiddleware(a.flyDevRobotsMiddleware(mux))
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
	PostalCode  sql.NullString `json:"postalCode"`
	CountryCode string         `json:"countryCode"`
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
	Images  []sqlcdb.Image
	Relics  []RelicWithImages
	Sources []string
}

type RelicWithImages struct {
	Relic  sqlcdb.ListRelicsForChurchRow
	Images []sqlcdb.Image
}

type ChurchWithRelicImages struct {
	Church sqlcdb.Church
	Images []sqlcdb.Image
}

type SaintWithType struct {
	Type            string
	Saint           sqlcdb.Saint
	Images          []sqlcdb.Image
	ReferringChurch *sqlcdb.Church
	Churches        []ChurchWithRelicImages
}

func (a *Application) homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=86400, stale-while-revalidate=86400")
	w.Header().Set("Vary", "Accept-Encoding, HX-Request")
	var data any
	var err error
	var metadata PageMetadata

	if r.URL.Path != "/" {
		path := r.URL.Path
		if strings.HasPrefix(path, "/churches/") {
			slug := path[10:]
			church, churchErr := a.DB.GetChurchBySlug(r.Context(), slug)
			if churchErr == nil {
				relicRows, _ := a.DB.ListRelicsForChurch(r.Context(), church.ID)
				sources, _ := a.DB.ListSourcesForChurch(r.Context(), church.ID)
				images, _ := a.DB.ListImagesForChurch(r.Context(), sql.NullInt64{Int64: church.ID, Valid: true})

				relics := make([]RelicWithImages, len(relicRows))
				for i, rRow := range relicRows {
					rImages, _ := a.DB.ListImagesForRelic(r.Context(), sqlcdb.ListImagesForRelicParams{
						RelicChurchID: sql.NullInt64{Int64: church.ID, Valid: true},
						RelicSaintID:  sql.NullInt64{Int64: rRow.ID, Valid: true},
					})
					relics[i] = RelicWithImages{
						Relic:  rRow,
						Images: rImages,
					}
				}

				data = ChurchWithRelics{
					Type:    "church",
					Church:  church,
					Images:  images,
					Relics:  relics,
					Sources: sources,
				}
				metadata = a.getChurchMetadata(church, relicRows)
			} else {
				err = churchErr
			}
		} else {
			// Check if it's a saint slug (paths like /st-seraphim-of-sarov)
			slug := strings.TrimPrefix(path, "/")
			saint, saintErr := a.DB.GetSaintBySlug(r.Context(), slug)
			if saintErr == nil {
				// Fetch all churches for this saint
				churchesRows, _ := a.DB.ListChurchesBySaintSlug(r.Context(), slug)
				images, _ := a.DB.ListImagesForSaint(r.Context(), sql.NullInt64{Int64: saint.ID, Valid: true})

				churches := make([]ChurchWithRelicImages, len(churchesRows))
				for i, c := range churchesRows {
					cImages, _ := a.DB.ListImagesForRelic(r.Context(), sqlcdb.ListImagesForRelicParams{
						RelicChurchID: sql.NullInt64{Int64: c.ID, Valid: true},
						RelicSaintID:  sql.NullInt64{Int64: saint.ID, Valid: true},
					})
					churches[i] = ChurchWithRelicImages{
						Church: c,
						Images: cImages,
					}
				}

				sData := SaintWithType{
					Type:     "saint",
					Saint:    saint,
					Images:   images,
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
				metadata = a.getSaintMetadata(saint)
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
			slog.Error("Error fetching data", "path", strings.NewReplacer("\n", "", "\r", "").Replace(path), "error", err) // nolint:gosec // G706: input is manually sanitized to remove newlines and prevent log injection
			return
		}
	} else {
		metadata = a.getBaseMetadata(r.URL.Path)
	}

	pageData := PageData{
		Metadata: metadata,
		Content:  data,
	}

	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("Cache-Control", "public, max-age=86400, stale-while-revalidate=86400")
		w.Header().Set("HX-Title", metadata.Title)
		q := r.URL.Query()
		q.Del("hx")
		pushURL := r.URL.Path
		if len(q) > 0 {
			pushURL += "?" + q.Encode()
		}
		w.Header().Set("HX-Push-Url", pushURL)

		// Set trigger headers for map interaction
		if data != nil {
			if s, ok := data.(SaintWithType); ok {
				trigger := map[string]any{
					"saintSelected": map[string]string{
						"slug": s.Saint.Slug,
					},
				}
				triggerJSON, _ := json.Marshal(trigger)
				w.Header().Set("HX-Trigger", string(triggerJSON))

				ts, err := a.Templates.Get("saint-detail")
				if err != nil {
					http.Error(w, "saint-detail template not found", http.StatusInternalServerError)
					return
				}
				err = ts.ExecuteTemplate(w, "saint-detail", s)
				if err != nil {
					slog.Error("Error rendering saint detail", "error", err)
				}
				return
			} else if c, ok := data.(ChurchWithRelics); ok {
				trigger := map[string]any{
					"churchSelected": map[string]any{
						"slug": c.Church.Slug,
						"lat":  c.Church.Latitude,
						"lng":  c.Church.Longitude,
					},
				}
				triggerJSON, _ := json.Marshal(trigger)
				w.Header().Set("HX-Trigger", string(triggerJSON))

				ts, err := a.Templates.Get("church-detail")
				if err != nil {
					http.Error(w, "church-detail template not found", http.StatusInternalServerError)
					return
				}
				err = ts.ExecuteTemplate(w, "church-detail", c)
				if err != nil {
					slog.Error("Error rendering church detail", "error", err)
				}
				return
			}
		} else if r.URL.Path == "/" {
			// Handle Map Reset / Home via HTMX
			trigger := map[string]any{
				"directorySelected": map[string]any{
					"title":        "Details",
					"closeSidebar": true,
				},
			}
			triggerJSON, _ := json.Marshal(trigger)
			w.Header().Set("HX-Trigger", string(triggerJSON))
			fmt.Fprint(w, `<div class="loading">Select a location on the map to begin your pilgrimage.</div>`)
			return
		}
	}

	w.Header().Set("Cache-Control", "public, max-age=86400, stale-while-revalidate=86400")
	if err := a.Templates.Render(w, "index", pageData); err != nil {
		http.Error(w, "failed to render template", http.StatusInternalServerError)
		slog.Error("Error rendering template", "error", err)
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
		slog.Error("Error retrieving churches", "error", err)
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
			PostalCode:  c.PostalCode,
			CountryCode: c.CountryCode,
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
		slog.Error("Error encoding churches", "error", err)
	}
}

func (a *Application) churchDetailHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	// Expecting /churches/ or /churches/{slug}
	if path == "/churches" || path == "/churches/" {
		a.churchesDirectoryHandler(w, r)
		return
	}

	if len(path) < 11 {
		http.NotFound(w, r)
		return
	}

	slug := path[10:]
	if slug == "" {
		a.churchesDirectoryHandler(w, r)
		return
	}

	church, err := a.DB.GetChurchBySlug(r.Context(), slug)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to retrieve church", http.StatusInternalServerError)
		slog.Error("Error retrieving church", "slug", strings.NewReplacer("\n", "", "\r", "").Replace(slug), "error", err) // nolint:gosec // G706: input is manually sanitized to remove newlines and prevent log injection
		return
	}

	relicRows, _ := a.DB.ListRelicsForChurch(r.Context(), church.ID)
	sources, _ := a.DB.ListSourcesForChurch(r.Context(), church.ID)
	images, _ := a.DB.ListImagesForChurch(r.Context(), sql.NullInt64{Int64: church.ID, Valid: true})

	relics := make([]RelicWithImages, len(relicRows))
	for i, rRow := range relicRows {
		rImages, _ := a.DB.ListImagesForRelic(r.Context(), sqlcdb.ListImagesForRelicParams{
			RelicChurchID: sql.NullInt64{Int64: church.ID, Valid: true},
			RelicSaintID:  sql.NullInt64{Int64: rRow.ID, Valid: true},
		})
		relics[i] = RelicWithImages{
			Relic:  rRow,
			Images: rImages,
		}
	}

	data := ChurchWithRelics{
		Type:    "church",
		Church:  church,
		Images:  images,
		Relics:  relics,
		Sources: sources,
	}

	q := r.URL.Query()
	q.Del("hx")
	pushURL := path
	if len(q) > 0 {
		pushURL += "?" + q.Encode()
	}
	w.Header().Set("HX-Push-Url", pushURL)
	w.Header().Set("Vary", "Accept-Encoding, HX-Request")

	metadata := a.getChurchMetadata(church, relicRows)

	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("Cache-Control", "public, max-age=86400, stale-while-revalidate=86400")
		w.Header().Set("HX-Title", metadata.Title)

		trigger := map[string]any{
			"churchSelected": map[string]any{
				"slug": church.Slug,
				"lat":  church.Latitude,
				"lng":  church.Longitude,
			},
		}
		triggerJSON, _ := json.Marshal(trigger)
		w.Header().Set("HX-Trigger", string(triggerJSON))

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
		pageData := PageData{
			Metadata: metadata,
			Content:  data,
		}
		if err := a.Templates.Render(w, "index", pageData); err != nil {
			http.Error(w, "failed to render template", http.StatusInternalServerError)
			slog.Error("Error rendering template", "error", err)
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
			slog.Error("Error encoding search results", "error", err)
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
		slog.Error("Error encoding search results", "error", err)
	}
}

func (a *Application) churchesDirectoryHandler(w http.ResponseWriter, r *http.Request) {
	churches, err := a.DB.ListChurches(r.Context())
	if err != nil {
		http.Error(w, "Failed to retrieve churches", http.StatusInternalServerError)
		return
	}

	data := struct {
		Type     string
		Churches []sqlcdb.Church
	}{
		Type:     "church-directory",
		Churches: churches,
	}

	metadata := a.getChurchesDirectoryMetadata()
	q := r.URL.Query()
	q.Del("hx")
	pushURL := "/churches/"
	if len(q) > 0 {
		pushURL += "?" + q.Encode()
	}
	w.Header().Set("HX-Push-Url", pushURL)
	w.Header().Set("Vary", "Accept-Encoding, HX-Request")

	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("Cache-Control", "public, max-age=86400, stale-while-revalidate=86400")
		w.Header().Set("HX-Title", metadata.Title)

		trigger := map[string]any{
			"directorySelected": map[string]string{
				"title": "All Churches",
			},
		}
		triggerJSON, _ := json.Marshal(trigger)
		w.Header().Set("HX-Trigger", string(triggerJSON))

		ts, err := a.Templates.Get("church-directory")
		if err != nil {
			http.Error(w, "church-directory template not found", http.StatusInternalServerError)
			return
		}
		err = ts.ExecuteTemplate(w, "church-directory", data)
		if err != nil {
			slog.Error("Error rendering church directory", "error", err)
		}
		return
	}

	pageData := PageData{
		Metadata: metadata,
		Content:  data,
	}

	w.Header().Set("Cache-Control", "public, max-age=86400, stale-while-revalidate=86400")
	if err := a.Templates.Render(w, "index", pageData); err != nil {
		http.Error(w, "failed to render template", http.StatusInternalServerError)
	}
}

func (a *Application) saintsDirectoryHandler(w http.ResponseWriter, r *http.Request) {
	saints, err := a.DB.ListSaints(r.Context())
	if err != nil {
		http.Error(w, "Failed to retrieve saints", http.StatusInternalServerError)
		return
	}

	data := struct {
		Type   string
		Saints []sqlcdb.Saint
	}{
		Type:   "saint-directory",
		Saints: saints,
	}

	metadata := a.getSaintsDirectoryMetadata()
	q := r.URL.Query()
	q.Del("hx")
	pushURL := "/saints/"
	if len(q) > 0 {
		pushURL += "?" + q.Encode()
	}
	w.Header().Set("HX-Push-Url", pushURL)
	w.Header().Set("Vary", "Accept-Encoding, HX-Request")

	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("Cache-Control", "public, max-age=86400, stale-while-revalidate=86400")
		w.Header().Set("HX-Title", metadata.Title)

		trigger := map[string]any{
			"directorySelected": map[string]string{
				"title": "All Saints",
			},
		}
		triggerJSON, _ := json.Marshal(trigger)
		w.Header().Set("HX-Trigger", string(triggerJSON))

		ts, err := a.Templates.Get("saint-directory")
		if err != nil {
			http.Error(w, "saint-directory template not found", http.StatusInternalServerError)
			return
		}
		err = ts.ExecuteTemplate(w, "saint-directory", data)
		if err != nil {
			slog.Error("Error rendering saint directory", "error", err)
		}
		return
	}

	pageData := PageData{
		Metadata: metadata,
		Content:  data,
	}

	w.Header().Set("Cache-Control", "public, max-age=86400, stale-while-revalidate=86400")
	if err := a.Templates.Render(w, "index", pageData); err != nil {
		http.Error(w, "failed to render template", http.StatusInternalServerError)
	}
}
