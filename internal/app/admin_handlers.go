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
	"github.com/pquerna/otp/totp"
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

		if !a.DevMode {
			if admin.MfaEnabled.Bool || admin.MfaSecret.Valid {
				a.SessionManager.Put(r.Context(), "mfa_pending", true)
				http.Redirect(w, r, "/admin/mfa", http.StatusSeeOther)
				return
			}
		}

		http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
	}
}

func (a *Application) adminMfaHandler(w http.ResponseWriter, r *http.Request) {
	if !a.SessionManager.Exists(r.Context(), "admin_id") {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		ts, err := a.Templates.Get("admin-mfa")
		if err != nil {
			slog.Error("Template not found", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		err = ts.ExecuteTemplate(w, "admin-mfa", nil)
		if err != nil {
			slog.Error("Error rendering template", "error", err)
		}
		return
	}

	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		code := r.FormValue("code")
		adminID := a.SessionManager.GetInt64(r.Context(), "admin_id")

		admin, err := a.DB.GetAdmin(r.Context(), adminID)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if !admin.MfaSecret.Valid {
			http.Error(w, "MFA Not Configured", http.StatusBadRequest)
			return
		}

		valid := totp.Validate(code, admin.MfaSecret.String)
		if !valid && !a.DevMode {
			ts, err := a.Templates.Get("admin-mfa")
			if err != nil {
				slog.Error("Template not found", "error", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			err = ts.ExecuteTemplate(w, "admin-mfa", map[string]string{"Error": "Invalid verification code"})
			if err != nil {
				slog.Error("Error rendering template", "error", err)
			}
			return
		}

		a.SessionManager.Remove(r.Context(), "mfa_pending")
		_ = a.DB.UpdateAdminLastLogin(r.Context(), admin.ID)
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

		key, err := totp.Generate(totp.GenerateOpts{
			Issuer:      "OrthodoxPilgrimage",
			AccountName: username,
		})
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		_, err = a.DB.CreateAdmin(r.Context(), sqlcdb.CreateAdminParams{
			Username:     username,
			PasswordHash: string(hash),
			MfaSecret:    sql.NullString{String: key.Secret(), Valid: true},
		})
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "Admin created! PLEASE SAVE YOUR MFA SECRET: %s\nThen go to /admin/login", key.Secret())
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
	var church sqlcdb.Church
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
		data := map[string]any{
			"Username":  a.SessionManager.GetString(r.Context(), "username"),
			"Church":    church,
			"IsNew":     isNew,
			"ActiveNav": "churches",
			"Title":     "Edit Church",
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
		jurisdiction := r.FormValue("jurisdiction")
		website := r.FormValue("website")
		phone := r.FormValue("phone")
		description := r.FormValue("description")
		status := r.FormValue("status")

		if slug == "" {
			slug = slugify(name)
		}

		lat, _ := strconv.ParseFloat(latStr, 64)
		lng, _ := strconv.ParseFloat(lngStr, 64)

		var updatedChurch sqlcdb.Church
		if isNew {
			updatedChurch, err = a.DB.CreateChurch(r.Context(), sqlcdb.CreateChurchParams{
				Name:          name,
				Slug:          slug,
				AddressText:   addressText,
				City:          city,
				StateProvince: stateProvince,
				PostalCode:    sql.NullString{String: postalCode, Valid: postalCode != ""},
				CountryCode:   countryCode,
				Latitude:      lat,
				Longitude:     lng,
				Jurisdiction:  sql.NullString{String: jurisdiction, Valid: jurisdiction != ""},
				Website:       sql.NullString{String: website, Valid: website != ""},
				Phone:         sql.NullString{String: phone, Valid: phone != ""},
				Description:   sql.NullString{String: description, Valid: description != ""},
				Status:        status,
			})
			if err == nil {
				a.logAudit(r.Context(), "CREATE", "church", updatedChurch.ID, updatedChurch)
			}
		} else {
			updatedChurch, err = a.DB.UpdateChurch(r.Context(), sqlcdb.UpdateChurchParams{
				ID:            church.ID,
				Name:          name,
				Slug:          slug,
				AddressText:   addressText,
				City:          city,
				StateProvince: stateProvince,
				PostalCode:    sql.NullString{String: postalCode, Valid: postalCode != ""},
				CountryCode:   countryCode,
				Latitude:      lat,
				Longitude:     lng,
				Jurisdiction:  sql.NullString{String: jurisdiction, Valid: jurisdiction != ""},
				Website:       sql.NullString{String: website, Valid: website != ""},
				Phone:         sql.NullString{String: phone, Valid: phone != ""},
				Description:   sql.NullString{String: description, Valid: description != ""},
				Status:        status,
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

		data := map[string]any{
			"Username":  a.SessionManager.GetString(r.Context(), "username"),
			"Saints":    saints,
			"Churches":  churches,
			"ActiveNav": "relics",
			"Title":     "Add Relic",
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

		err := a.DB.CreateRelic(r.Context(), sqlcdb.CreateRelicParams{
			ChurchID:    churchID,
			SaintID:     saintID,
			Description: sql.NullString{String: description, Valid: description != ""},
		})

		if err != nil {
			slog.Error("Failed to save relic", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		a.logAudit(r.Context(), "CREATE", "relic", churchID, map[string]int64{"church_id": churchID, "saint_id": saintID})

		w.Header().Set("HX-Location", "/admin/relics")
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
