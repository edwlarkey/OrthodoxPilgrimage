package app

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type URL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

type URLSet struct {
	XMLName xml.Name `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
	URLs    []URL    `xml:"url"`
}

func (a *Application) sitemapHandler(w http.ResponseWriter, r *http.Request) {
	baseURL := "https://orthodoxpilgrimage.com"
	ctx := r.Context()

	churches, err := a.DB.ListChurches(ctx)
	if err != nil {
		slog.Error("failed to list churches for sitemap", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	saints, err := a.DB.ListSaints(ctx)
	if err != nil {
		slog.Error("failed to list saints for sitemap", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	urlSet := URLSet{
		URLs: []URL{
			{Loc: baseURL + "/", LastMod: time.Now().Format("2006-01-02")},
		},
	}

	for _, c := range churches {
		lastMod := time.Now().Format("2006-01-02")
		if c.UpdatedAt.Valid {
			lastMod = c.UpdatedAt.Time.Format("2006-01-02")
		}
		urlSet.URLs = append(urlSet.URLs, URL{
			Loc:     fmt.Sprintf("%s/churches/%s", baseURL, c.Slug),
			LastMod: lastMod,
		})
	}

	for _, s := range saints {
		lastMod := time.Now().Format("2006-01-02")
		if s.UpdatedAt.Valid {
			lastMod = s.UpdatedAt.Time.Format("2006-01-02")
		}
		urlSet.URLs = append(urlSet.URLs, URL{
			Loc:     fmt.Sprintf("%s/%s", baseURL, s.Slug),
			LastMod: lastMod,
		})
	}

	w.Header().Set("Content-Type", "application/xml")
	// Cache the sitemap for 1 hour to reduce DB load
	w.Header().Set("Cache-Control", "public, max-age=3600")

	if _, err := w.Write([]byte(xml.Header)); err != nil {
		slog.Error("failed to write xml header for sitemap", "error", err)
		return
	}

	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(urlSet); err != nil {
		slog.Error("failed to encode sitemap", "error", err)
	}
}
