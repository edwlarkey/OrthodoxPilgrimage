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
	"golang.org/x/crypto/bcrypt"
)

func (a *Application) adminLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		var adminCount int
		err := a.DBConn.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM admins").Scan(&adminCount)
		if err != nil {
			slog.Error("Failed to count admins", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if adminCount == 0 {
			http.Redirect(w, r, "/admin/setup", http.StatusSeeOther)
			return
		}

		ts, err := a.Templates.Get("admin-login")
		if err != nil {
			slog.Error("Template not found", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		err = ts.ExecuteTemplate(w, "admin-login", nil)
		if err != nil {
			slog.Error("Error rendering template", "error", err)
		}
		return
	}

	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		username := r.FormValue("username")
		password := r.FormValue("password")

		admin, err := a.DB.GetAdminByUsername(r.Context(), username)
		if err != nil {
			if err == sql.ErrNoRows {
				ts, err := a.Templates.Get("admin-login")
				if err != nil {
					slog.Error("Template not found", "error", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				err = ts.ExecuteTemplate(w, "admin-login", map[string]string{"Error": "Invalid username or password"})
				if err != nil {
					slog.Error("Error rendering template", "error", err)
				}
				return
			}
			slog.Error("Database error during login", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password))
		if err != nil {
			ts, err := a.Templates.Get("admin-login")
			if err != nil {
				slog.Error("Template not found", "error", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			err = ts.ExecuteTemplate(w, "admin-login", map[string]string{"Error": "Invalid username or password"})
			if err != nil {
				slog.Error("Error rendering template", "error", err)
			}
			return
		}

		a.SessionManager.Put(r.Context(), "admin_id", admin.ID)
		a.SessionManager.Put(r.Context(), "username", admin.Username)

		http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
	}
}

func (a *Application) adminDashboardHandler(w http.ResponseWriter, r *http.Request) {
	churchCount, _ := a.DB.CountChurches(r.Context())
	saintCount, _ := a.DB.CountSaints(r.Context())
	relicCount, _ := a.DB.CountRelics(r.Context())

	recentChurches, _ := a.DB.ListRecentChurches(r.Context())
	recentSaints, _ := a.DB.ListRecentSaints(r.Context())
	recentRelics, _ := a.DB.ListRecentRelics(r.Context())

	data := map[string]any{
		"Username":       a.SessionManager.GetString(r.Context(), "username"),
		"ChurchCount":    churchCount,
		"SaintCount":     saintCount,
		"RelicCount":     relicCount,
		"RecentChurches": recentChurches,
		"RecentSaints":   recentSaints,
		"RecentRelics":   recentRelics,
		"ActiveNav":      "dashboard",
		"Title":          "Dashboard",
	}

	ts, err := a.Templates.Get("admin-dashboard")
	if err != nil {
		slog.Error("Template not found", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	err = ts.ExecuteTemplate(w, "admin-dashboard", data)
	if err != nil {
		slog.Error("Error rendering admin dashboard", "error", err)
	}
}

func (a *Application) adminLogoutHandler(w http.ResponseWriter, r *http.Request) {
	_ = a.SessionManager.Destroy(r.Context())
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

func (a *Application) adminSetupHandler(w http.ResponseWriter, r *http.Request) {
	var adminCount int
	err := a.DBConn.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM admins").Scan(&adminCount)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if adminCount > 0 {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method == http.MethodGet {
		ts, err := a.Templates.Get("admin-login")
		if err != nil {
			slog.Error("Template not found", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		err = ts.ExecuteTemplate(w, "admin-login", map[string]any{
			"Setup": true,
			"Title": "Create First Admin",
		})
		if err != nil {
			slog.Error("Error rendering template", "error", err)
		}
		return
	}

	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		username := r.FormValue("username")
		password := r.FormValue("password")

		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		_, err = a.DB.CreateAdmin(r.Context(), sqlcdb.CreateAdminParams{
			Username:     username,
			PasswordHash: string(hash),
		})
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "Admin created! Then go to /admin/login")
	}
}

func (a *Application) logAudit(ctx context.Context, action, entityType string, entityID int64, changes any) {
	adminID := a.SessionManager.GetInt64(ctx, "admin_id")
	if adminID == 0 {
		slog.Warn("Attempted to log audit without admin_id in session", "action", action)
		return
	}

	var changesStr string
	if changes != nil {
		b, err := json.Marshal(changes)
		if err == nil {
			changesStr = string(b)
		}
	}

	slog.Info("Audit Log",
		"admin_id", adminID,
		"action", action,
		"entity_type", entityType,
		"entity_id", entityID,
		"changes", changesStr,
	)
}

func (a *Application) adminSaintsListHandler(w http.ResponseWriter, r *http.Request) {
	saints, err := a.DB.ListSaints(r.Context())
	if err != nil {
		slog.Error("Failed to list saints", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Username":  a.SessionManager.GetString(r.Context(), "username"),
		"Saints":    saints,
		"ActiveNav": "saints",
		"Title":     "Saints Management",
	}

	ts, err := a.Templates.Get("admin-saints-list")
	if err != nil {
		slog.Error("Template not found", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	err = ts.ExecuteTemplate(w, "admin-saints-list", data)
	if err != nil {
		slog.Error("Error rendering saints list", "error", err)
	}
}

func (a *Application) adminSaintEditHandler(w http.ResponseWriter, r *http.Request) {
	var saint sqlcdb.Saint
	var err error
	isNew := true

	path := r.URL.Path
	if strings.HasPrefix(path, "/admin/saints/edit/") {
		slug := strings.TrimPrefix(path, "/admin/saints/edit/")
		saint, err = a.DB.GetSaintBySlug(r.Context(), slug)
		if err != nil {
			if err == sql.ErrNoRows {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		isNew = false
	}

	if r.Method == http.MethodGet {
		data := map[string]any{
			"Username":  a.SessionManager.GetString(r.Context(), "username"),
			"Saint":     saint,
			"IsNew":     isNew,
			"ActiveNav": "saints",
			"Title":     "Edit Saint",
		}
		if isNew {
			data["Title"] = "New Saint"
		}

		ts, err := a.Templates.Get("admin-saints-edit")
		if err != nil {
			slog.Error("Template not found", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		err = ts.ExecuteTemplate(w, "admin-saints-edit", data)
		if err != nil {
			slog.Error("Error rendering saint edit", "error", err)
		}
		return
	}

	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 1048576)
		name := r.FormValue("name")
		slug := r.FormValue("slug")
		feastDay := r.FormValue("feast_day")
		description := r.FormValue("description")
		livesUrl := r.FormValue("lives_url")
		status := r.FormValue("status")

		if slug == "" {
			slug = slugify(name)
		}

		if name == "" {
			http.Error(w, "Name is required", http.StatusBadRequest)
			return
		}

		var updatedSaint sqlcdb.Saint
		if isNew {
			updatedSaint, err = a.DB.CreateSaint(r.Context(), sqlcdb.CreateSaintParams{
				Name:        name,
				Slug:        slug,
				FeastDay:    sql.NullString{String: feastDay, Valid: feastDay != ""},
				Description: sql.NullString{String: description, Valid: description != ""},
				LivesUrl:    sql.NullString{String: livesUrl, Valid: livesUrl != ""},
				Status:      status,
			})
			if err == nil {
				a.logAudit(r.Context(), "CREATE", "saint", updatedSaint.ID, updatedSaint)
			}
		} else {
			updatedSaint, err = a.DB.UpdateSaint(r.Context(), sqlcdb.UpdateSaintParams{
				ID:          saint.ID,
				Name:        name,
				Slug:        slug,
				FeastDay:    sql.NullString{String: feastDay, Valid: feastDay != ""},
				Description: sql.NullString{String: description, Valid: description != ""},
				LivesUrl:    sql.NullString{String: livesUrl, Valid: livesUrl != ""},
				Status:      status,
			})
			if err == nil {
				a.logAudit(r.Context(), "UPDATE", "saint", saint.ID, updatedSaint)
			}
		}

		if err != nil {
			slog.Error("Failed to save saint", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("HX-Location", "/admin/saints")
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *Application) adminSaintDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	slug := strings.TrimPrefix(r.URL.Path, "/admin/saints/delete/")
	saint, err := a.DB.GetSaintBySlug(r.Context(), slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	err = a.DB.DeleteSaint(r.Context(), saint.ID)
	if err != nil {
		slog.Error("Failed to delete saint", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	a.logAudit(r.Context(), "DELETE", "saint", saint.ID, nil)

	if r.Header.Get("HX-Request") != "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	http.Redirect(w, r, "/admin/saints", http.StatusSeeOther)
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, ".", "")
	return s
}

func (a *Application) adminChurchesListHandler(w http.ResponseWriter, r *http.Request) {
	churches, err := a.DB.ListChurches(r.Context())
	if err != nil {
		slog.Error("Failed to list churches", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Username":  a.SessionManager.GetString(r.Context(), "username"),
		"Churches":  churches,
		"ActiveNav": "churches",
		"Title":     "Churches Management",
	}

	ts, err := a.Templates.Get("admin-churches-list")
	if err != nil {
		slog.Error("Template not found", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	err = ts.ExecuteTemplate(w, "admin-churches-list", data)
	if err != nil {
		slog.Error("Error rendering churches list", "error", err)
	}
}

func (a *Application) adminChurchEditHandler(w http.ResponseWriter, r *http.Request) {
	var church sqlcdb.GetChurchBySlugRow
	var err error
	isNew := true

	path := r.URL.Path
	if strings.HasPrefix(path, "/admin/churches/edit/") {
		slug := strings.TrimPrefix(path, "/admin/churches/edit/")
		church, err = a.DB.GetChurchBySlug(r.Context(), slug)
		if err != nil {
			if err == sql.ErrNoRows {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		isNew = false
	}

	if r.Method == http.MethodGet {
		var relics []RelicWithImages
		var saints []sqlcdb.Saint
		var sources []sqlcdb.ChurchSource
		jurisdictions, _ := a.DB.ListJurisdictions(r.Context())
		relicTypes, _ := a.DB.ListRelicTypes(r.Context())

		if !isNew {
			relicRows, _ := a.DB.ListRelicsForChurch(r.Context(), church.ID)
			saints, _ = a.DB.ListSaints(r.Context())
			sources, _ = a.DB.ListSourcesForChurch(r.Context(), church.ID)

			relics = make([]RelicWithImages, len(relicRows))
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
		}

		data := map[string]any{
			"Username":      a.SessionManager.GetString(r.Context(), "username"),
			"Church":        church,
			"IsNew":         isNew,
			"Relics":        relics,
			"Saints":        saints,
			"Sources":       sources,
			"Jurisdictions": jurisdictions,
			"RelicTypes":    relicTypes,
			"ActiveNav":     "churches",
			"Title":         "Edit Church",
		}
		if isNew {
			data["Title"] = "New Church"
		}

		ts, err := a.Templates.Get("admin-churches-edit")
		if err != nil {
			slog.Error("Template not found", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		err = ts.ExecuteTemplate(w, "admin-churches-edit", data)
		if err != nil {
			slog.Error("Error rendering church edit", "error", err)
		}
		return
	}

	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 1048576)
		name := r.FormValue("name")
		slug := r.FormValue("slug")
		addressText := r.FormValue("address_text")
		city := r.FormValue("city")
		stateProvince := r.FormValue("state_province")
		postalCode := r.FormValue("postal_code")
		countryCode := r.FormValue("country_code")
		latStr := r.FormValue("latitude")
		lngStr := r.FormValue("longitude")
		jurisdictionIDStr := r.FormValue("jurisdiction_id")
		website := r.FormValue("website")
		phone := r.FormValue("phone")
		description := r.FormValue("description")
		status := r.FormValue("status")

		if slug == "" {
			slug = slugify(name)
		}

		lat, _ := strconv.ParseFloat(latStr, 64)
		lng, _ := strconv.ParseFloat(lngStr, 64)

		var jurisdictionID sql.NullInt64
		if jurisdictionIDStr != "" {
			if id, err := strconv.ParseInt(jurisdictionIDStr, 10, 64); err == nil {
				jurisdictionID = sql.NullInt64{Int64: id, Valid: true}
			}
		}

		var updatedChurch sqlcdb.Church
		if isNew {
			updatedChurch, err = a.DB.CreateChurch(r.Context(), sqlcdb.CreateChurchParams{
				Name:           name,
				Slug:           slug,
				AddressText:    addressText,
				City:           city,
				StateProvince:  stateProvince,
				PostalCode:     sql.NullString{String: postalCode, Valid: postalCode != ""},
				CountryCode:    countryCode,
				Latitude:       lat,
				Longitude:      lng,
				JurisdictionID: jurisdictionID,
				Website:        sql.NullString{String: website, Valid: website != ""},
				Phone:          sql.NullString{String: phone, Valid: phone != ""},
				Description:    sql.NullString{String: description, Valid: description != ""},
				Status:         status,
			})
			if err == nil {
				a.logAudit(r.Context(), "CREATE", "church", updatedChurch.ID, updatedChurch)
			}
		} else {
			updatedChurch, err = a.DB.UpdateChurch(r.Context(), sqlcdb.UpdateChurchParams{
				ID:             church.ID,
				Name:           name,
				Slug:           slug,
				AddressText:    addressText,
				City:           city,
				StateProvince:  stateProvince,
				PostalCode:     sql.NullString{String: postalCode, Valid: postalCode != ""},
				CountryCode:    countryCode,
				Latitude:       lat,
				Longitude:      lng,
				JurisdictionID: jurisdictionID,
				Website:        sql.NullString{String: website, Valid: website != ""},
				Phone:          sql.NullString{String: phone, Valid: phone != ""},
				Description:    sql.NullString{String: description, Valid: description != ""},
				Status:         status,
			})
			if err == nil {
				a.logAudit(r.Context(), "UPDATE", "church", church.ID, updatedChurch)
			}
		}

		if err != nil {
			slog.Error("Failed to save church", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("HX-Location", "/admin/churches")
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *Application) adminChurchDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	slug := strings.TrimPrefix(r.URL.Path, "/admin/churches/delete/")
	church, err := a.DB.GetChurchBySlug(r.Context(), slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	err = a.DB.DeleteChurch(r.Context(), church.ID)
	if err != nil {
		slog.Error("Failed to delete church", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	a.logAudit(r.Context(), "DELETE", "church", church.ID, nil)

	if r.Header.Get("HX-Request") != "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	http.Redirect(w, r, "/admin/churches", http.StatusSeeOther)
}

func (a *Application) adminRelicsListHandler(w http.ResponseWriter, r *http.Request) {
	relics, err := a.DB.ListAllRelics(r.Context())
	if err != nil {
		slog.Error("Failed to list relics", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Username":  a.SessionManager.GetString(r.Context(), "username"),
		"Relics":    relics,
		"ActiveNav": "relics",
		"Title":     "Relics Management",
	}

	ts, err := a.Templates.Get("admin-relics-list")
	if err != nil {
		slog.Error("Template not found", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	err = ts.ExecuteTemplate(w, "admin-relics-list", data)
	if err != nil {
		slog.Error("Error rendering relics list", "error", err)
	}
}

func (a *Application) adminRelicEditHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		saints, _ := a.DB.ListSaints(r.Context())
		churches, _ := a.DB.ListChurches(r.Context())
		relicTypes, _ := a.DB.ListRelicTypes(r.Context())

		data := map[string]any{
			"Username":   a.SessionManager.GetString(r.Context(), "username"),
			"Saints":     saints,
			"Churches":   churches,
			"RelicTypes": relicTypes,
			"ActiveNav":  "relics",
			"Title":      "Add Relic",
		}

		ts, err := a.Templates.Get("admin-relics-edit")
		if err != nil {
			slog.Error("Template not found", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		err = ts.ExecuteTemplate(w, "admin-relics-edit", data)
		if err != nil {
			slog.Error("Error rendering relic edit", "error", err)
		}
		return
	}

	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		churchID, _ := strconv.ParseInt(r.FormValue("church_id"), 10, 64)
		saintID, _ := strconv.ParseInt(r.FormValue("saint_id"), 10, 64)
		description := r.FormValue("description")

		var relicTypeID sql.NullInt64
		if rtIDStr := r.FormValue("relic_type_id"); rtIDStr != "" {
			if id, err := strconv.ParseInt(rtIDStr, 10, 64); err == nil {
				relicTypeID = sql.NullInt64{Int64: id, Valid: true}
			}
		}

		err := a.DB.CreateRelic(r.Context(), sqlcdb.CreateRelicParams{
			ChurchID:    churchID,
			SaintID:     saintID,
			RelicTypeID: relicTypeID,
			Description: sql.NullString{String: description, Valid: description != ""},
		})

		if err != nil {
			slog.Error("Failed to save relic", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		a.logAudit(r.Context(), "CREATE", "relic", churchID, map[string]int64{"church_id": churchID, "saint_id": saintID})

		redirectUrl := r.Header.Get("HX-Current-URL")
		if redirectUrl == "" {
			redirectUrl = "/admin/relics"
		}
		w.Header().Set("HX-Location", redirectUrl)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *Application) adminRelicUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		churchID, _ := strconv.ParseInt(r.FormValue("church_id"), 10, 64)
		saintID, _ := strconv.ParseInt(r.FormValue("saint_id"), 10, 64)
		description := r.FormValue("description")

		var relicTypeID sql.NullInt64
		if rtIDStr := r.FormValue("relic_type_id"); rtIDStr != "" {
			if id, err := strconv.ParseInt(rtIDStr, 10, 64); err == nil {
				relicTypeID = sql.NullInt64{Int64: id, Valid: true}
			}
		}

		err := a.DB.UpdateRelic(r.Context(), sqlcdb.UpdateRelicParams{
			ChurchID:    churchID,
			SaintID:     saintID,
			RelicTypeID: relicTypeID,
			Description: sql.NullString{String: description, Valid: description != ""},
		})

		if err != nil {
			slog.Error("Failed to update relic", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		a.logAudit(r.Context(), "UPDATE", "relic", churchID, map[string]int64{"church_id": churchID, "saint_id": saintID})

		w.Header().Set("HX-Trigger", `{"adminToast": {"message": "Relic updated successfully.", "type": "success"}}`)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *Application) adminRelicDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	churchID, _ := strconv.ParseInt(r.URL.Query().Get("church_id"), 10, 64)
	saintID, _ := strconv.ParseInt(r.URL.Query().Get("saint_id"), 10, 64)

	err := a.DB.DeleteRelic(r.Context(), sqlcdb.DeleteRelicParams{
		ChurchID: churchID,
		SaintID:  saintID,
	})
	if err != nil {
		slog.Error("Failed to delete relic", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	a.logAudit(r.Context(), "DELETE", "relic", churchID, map[string]int64{"church_id": churchID, "saint_id": saintID})

	if r.Header.Get("HX-Request") != "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	http.Redirect(w, r, "/admin/relics", http.StatusSeeOther)
}

func (a *Application) adminChurchSourceAddHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	churchID, _ := strconv.ParseInt(r.FormValue("church_id"), 10, 64)
	source := r.FormValue("source")
	if source == "" {
		http.Error(w, "Source is required", http.StatusBadRequest)
		return
	}

	err := a.DB.CreateChurchSource(r.Context(), sqlcdb.CreateChurchSourceParams{
		ChurchID: churchID,
		Source:   source,
	})
	if err != nil {
		slog.Error("Failed to add church source", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	a.logAudit(r.Context(), "CREATE", "church_source", churchID, map[string]any{"church_id": churchID, "source": source})

	w.Header().Set("HX-Location", r.Header.Get("HX-Current-URL"))
	w.WriteHeader(http.StatusNoContent)
}

func (a *Application) adminChurchSourceDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)

	err := a.DB.DeleteChurchSource(r.Context(), id)
	if err != nil {
		slog.Error("Failed to delete church source", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	a.logAudit(r.Context(), "DELETE", "church_source", id, nil)

	w.Header().Set("HX-Location", r.Header.Get("HX-Current-URL"))
	w.WriteHeader(http.StatusNoContent)
}

func (a *Application) adminListAdminsHandler(w http.ResponseWriter, r *http.Request) {
	admins, err := a.DB.ListAdmins(r.Context())
	if err != nil {
		slog.Error("Failed to list admins", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		ActiveNav string
		Title     string
		Username  string
		Admins    []sqlcdb.Admin
	}{
		ActiveNav: "admins",
		Title:     "Admins",
		Username:  a.SessionManager.GetString(r.Context(), "username"),
		Admins:    admins,
	}

	ts, err := a.Templates.Get("admin-admins-list")
	if err != nil {
		slog.Error("Template not found", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	err = ts.ExecuteTemplate(w, "admin-admins-list", data)
	if err != nil {
		slog.Error("Error rendering template", "error", err)
	}
}

func (a *Application) adminCreateAdminHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		ts, err := a.Templates.Get("admin-admins-new")
		if err != nil {
			slog.Error("Template not found", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		err = ts.ExecuteTemplate(w, "admin-admins-new", nil)
		if err != nil {
			slog.Error("Error rendering template", "error", err)
		}
		return
	}

	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")

		if username == "" || len(password) < 8 {
			ts, _ := a.Templates.Get("admin-admins-new")
			err := ts.ExecuteTemplate(w, "admin-admins-new", map[string]string{"Error": "Username required and password must be at least 8 characters.", "Username": username})
			if err != nil {
				slog.Error("Error rendering template", "error", err)
			}
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
		if err != nil {
			slog.Error("Failed to hash password", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		admin, err := a.DB.CreateAdmin(r.Context(), sqlcdb.CreateAdminParams{
			Username:     username,
			PasswordHash: string(hash),
		})

		if err != nil {
			slog.Error("Failed to create admin", "error", err)
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				ts, _ := a.Templates.Get("admin-admins-new")
				err = ts.ExecuteTemplate(w, "admin-admins-new", map[string]string{"Error": "Username already exists.", "Username": username})
				if err != nil {
					slog.Error("Error rendering template", "error", err)
				}
				return
			}
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		a.logAudit(r.Context(), "CREATE", "admin", admin.ID, nil)

		w.Header().Set("HX-Trigger", `{"adminToast": {"message": "Admin created successfully.", "type": "success"}}`)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *Application) adminDeleteAdminHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// Prevent self-deletion
	currentAdminID := a.SessionManager.GetInt64(r.Context(), "admin_id")
	if id == currentAdminID {
		w.Header().Set("HX-Trigger", `{"adminToast": {"message": "You cannot delete your own account.", "type": "error"}}`)
		w.WriteHeader(http.StatusNoContent) // Just swallow it, or we could return 400
		return
	}

	err = a.DB.DeleteAdmin(r.Context(), id)
	if err != nil {
		slog.Error("Failed to delete admin", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	a.logAudit(r.Context(), "DELETE", "admin", id, nil)

	w.Header().Set("HX-Trigger", `{"adminToast": {"message": "Admin deleted successfully.", "type": "success"}}`)
	w.WriteHeader(http.StatusOK) // Just empty response, outerHTML swap will remove the row
}

func (a *Application) adminJurisdictionsListHandler(w http.ResponseWriter, r *http.Request) {
	jurisdictions, err := a.DB.ListJurisdictions(r.Context())
	if err != nil {
		slog.Error("Failed to list jurisdictions", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Username":      a.SessionManager.GetString(r.Context(), "username"),
		"Jurisdictions": jurisdictions,
		"ActiveNav":     "jurisdictions",
		"Title":         "Jurisdictions Management",
	}

	ts, err := a.Templates.Get("admin-jurisdictions-list")
	if err != nil {
		slog.Error("Template not found", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	err = ts.ExecuteTemplate(w, "admin-jurisdictions-list", data)
	if err != nil {
		slog.Error("Error rendering jurisdictions list", "error", err)
	}
}

func (a *Application) adminJurisdictionEditHandler(w http.ResponseWriter, r *http.Request) {
	var jurisdiction sqlcdb.Jurisdiction
	var err error
	isNew := true

	path := r.URL.Path
	if strings.HasPrefix(path, "/admin/jurisdictions/edit/") {
		idStr := strings.TrimPrefix(path, "/admin/jurisdictions/edit/")
		id, _ := strconv.ParseInt(idStr, 10, 64)
		jurisdiction, err = a.DB.GetJurisdiction(r.Context(), id)
		if err != nil {
			if err == sql.ErrNoRows {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		isNew = false
	}

	if r.Method == http.MethodGet {
		data := map[string]any{
			"Username":     a.SessionManager.GetString(r.Context(), "username"),
			"Jurisdiction": jurisdiction,
			"IsNew":        isNew,
			"ActiveNav":    "jurisdictions",
			"Title":        "Edit Jurisdiction",
		}
		if isNew {
			data["Title"] = "New Jurisdiction"
		}

		ts, err := a.Templates.Get("admin-jurisdictions-edit")
		if err != nil {
			slog.Error("Template not found", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		err = ts.ExecuteTemplate(w, "admin-jurisdictions-edit", data)
		if err != nil {
			slog.Error("Error rendering jurisdiction edit", "error", err)
		}
		return
	}

	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		name := r.FormValue("name")
		tradition := r.FormValue("tradition")
		pinColor := r.FormValue("pin_color")

		if name == "" || tradition == "" {
			http.Error(w, "Name and Tradition are required", http.StatusBadRequest)
			return
		}

		if isNew {
			j, err := a.DB.CreateJurisdiction(r.Context(), sqlcdb.CreateJurisdictionParams{
				Name:      name,
				Tradition: tradition,
				PinColor:  pinColor,
			})
			if err == nil {
				a.logAudit(r.Context(), "CREATE", "jurisdiction", j.ID, map[string]string{"name": name})
			} else {
				slog.Error("Failed to save new jurisdiction", "error", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
		} else {
			_, err = a.DB.UpdateJurisdiction(r.Context(), sqlcdb.UpdateJurisdictionParams{
				ID:        jurisdiction.ID,
				Name:      name,
				Tradition: tradition,
				PinColor:  pinColor,
			})
			if err == nil {
				a.logAudit(r.Context(), "UPDATE", "jurisdiction", jurisdiction.ID, map[string]string{"name": name})
			} else {
				slog.Error("Failed to update jurisdiction", "error", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
		}

		redirectUrl := "/admin/jurisdictions"
		w.Header().Set("HX-Location", redirectUrl)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *Application) adminJurisdictionDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	err = a.DB.DeleteJurisdiction(r.Context(), id)
	if err != nil {
		slog.Error("Failed to delete jurisdiction", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	a.logAudit(r.Context(), "DELETE", "jurisdiction", id, nil)

	w.Header().Set("HX-Trigger", `{"adminToast": {"message": "Jurisdiction deleted successfully.", "type": "success"}}`)
	w.WriteHeader(http.StatusOK)
}

func (a *Application) adminRelicTypesListHandler(w http.ResponseWriter, r *http.Request) {
	relicTypes, err := a.DB.ListRelicTypes(r.Context())
	if err != nil {
		slog.Error("Failed to list relic types", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Username":   a.SessionManager.GetString(r.Context(), "username"),
		"RelicTypes": relicTypes,
		"ActiveNav":  "relic-types",
		"Title":      "Relic Types Management",
	}

	ts, err := a.Templates.Get("admin-relic-types-list")
	if err != nil {
		slog.Error("Template not found", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	err = ts.ExecuteTemplate(w, "admin-relic-types-list", data)
	if err != nil {
		slog.Error("Error rendering relic types list", "error", err)
	}
}

func (a *Application) adminRelicTypeEditHandler(w http.ResponseWriter, r *http.Request) {
	var relicType sqlcdb.RelicType
	var err error
	isNew := true

	path := r.URL.Path
	if strings.HasPrefix(path, "/admin/relic-types/edit/") {
		idStr := strings.TrimPrefix(path, "/admin/relic-types/edit/")
		id, _ := strconv.ParseInt(idStr, 10, 64)
		relicType, err = a.DB.GetRelicType(r.Context(), id)
		if err != nil {
			if err == sql.ErrNoRows {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		isNew = false
	}

	if r.Method == http.MethodGet {
		data := map[string]any{
			"Username":  a.SessionManager.GetString(r.Context(), "username"),
			"RelicType": relicType,
			"IsNew":     isNew,
			"ActiveNav": "relic-types",
			"Title":     "Edit Relic Type",
		}
		if isNew {
			data["Title"] = "New Relic Type"
		}

		ts, err := a.Templates.Get("admin-relic-types-edit")
		if err != nil {
			slog.Error("Template not found", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		err = ts.ExecuteTemplate(w, "admin-relic-types-edit", data)
		if err != nil {
			slog.Error("Error rendering relic type edit", "error", err)
		}
		return
	}

	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		name := r.FormValue("name")
		sortOrderStr := r.FormValue("sort_order")
		sortOrder, _ := strconv.ParseInt(sortOrderStr, 10, 64)

		if name == "" {
			http.Error(w, "Name is required", http.StatusBadRequest)
			return
		}

		if isNew {
			rt, err := a.DB.CreateRelicType(r.Context(), sqlcdb.CreateRelicTypeParams{
				Name:      name,
				SortOrder: sortOrder,
			})
			if err == nil {
				a.logAudit(r.Context(), "CREATE", "relic_type", rt.ID, map[string]any{"name": name, "sort_order": sortOrder})
			} else {
				slog.Error("Failed to save new relic type", "error", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
		} else {
			_, err = a.DB.UpdateRelicType(r.Context(), sqlcdb.UpdateRelicTypeParams{
				ID:        relicType.ID,
				Name:      name,
				SortOrder: sortOrder,
			})
			if err == nil {
				a.logAudit(r.Context(), "UPDATE", "relic_type", relicType.ID, map[string]any{"name": name, "sort_order": sortOrder})
			} else {
				slog.Error("Failed to update relic type", "error", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
		}

		redirectUrl := "/admin/relic-types"
		w.Header().Set("HX-Location", redirectUrl)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *Application) adminRelicTypeDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	err = a.DB.DeleteRelicType(r.Context(), id)
	if err != nil {
		slog.Error("Failed to delete relic type", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	a.logAudit(r.Context(), "DELETE", "relic_type", id, nil)

	w.Header().Set("HX-Trigger", `{"adminToast": {"message": "Relic Type deleted successfully.", "type": "success"}}`)
	w.WriteHeader(http.StatusOK)
}
