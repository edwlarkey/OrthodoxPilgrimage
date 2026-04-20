package app

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"time"

	sqlcdb "github.com/edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
)

type URL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

type URLSet struct {
	XMLName xml.Name `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
	URLs    []URL    `xml:"url"`
}

func GenerateSitemap(ctx context.Context, queries *sqlcdb.Queries, baseURL string) error {
	churches, err := queries.ListChurches(ctx)
	if err != nil {
		return fmt.Errorf("failed to list churches: %w", err)
	}

	saints, err := queries.ListSaints(ctx)
	if err != nil {
		return fmt.Errorf("failed to list saints: %w", err)
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

	f, err := os.Create("sitemap.xml")
	if err != nil {
		return fmt.Errorf("failed to create sitemap file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write([]byte(xml.Header)); err != nil {
		return err
	}

	enc := xml.NewEncoder(f)
	enc.Indent("", "  ")
	if err := enc.Encode(urlSet); err != nil {
		return fmt.Errorf("failed to encode sitemap: %w", err)
	}

	return nil
}
