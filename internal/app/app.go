package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	sqlcdb "git.sr.ht/~edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
	"git.sr.ht/~edwlarkey/orthodoxpilgrimage/internal/ui"
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
	mux.HandleFunc("/churches/", a.churchDetailHandler)
	return mux
}

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

func (a *Application) homeHandler(w http.ResponseWriter, r *http.Request) {
	var data interface{}
	var err error

	if r.URL.Path != "/" {
		var id int64
		if _, err = fmt.Sscanf(r.URL.Path, "/churches/%d", &id); err == nil {
			data, err = a.DB.GetChurch(r.Context(), id)
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
	a.Templates.Render(w, "index", data)
}

func (a *Application) listChurchesHandler(w http.ResponseWriter, r *http.Request) {
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
		churches, err := a.DB.ListChurchesInBounds(ctx, params)
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

	churches, err := a.DB.ListChurches(ctx)
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

func (a *Application) churchDetailHandler(w http.ResponseWriter, r *http.Request) {
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

	church, err := a.DB.GetChurch(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to retrieve church", http.StatusInternalServerError)
		log.Printf("Error retrieving church %d: %v", id, err)
		return
	}

	w.Header().Set("HX-Push-Url", fmt.Sprintf("/churches/%d", id))
	if r.Header.Get("HX-Request") != "" {
		ts, err := a.Templates.Get("church-detail")
		if err != nil {
			http.Error(w, "church-detail template not found", http.StatusInternalServerError)
			return
		}
		err = ts.ExecuteTemplate(w, "church-detail", church)
		if err != nil {
			http.Error(w, "failed to render church detail", http.StatusInternalServerError)
		}
	} else {
		a.Templates.Render(w, "index", church)
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
