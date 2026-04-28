package app

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"strings"

	sqlcdb "github.com/edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
)

type PageMetadata struct {
	Title          string
	Description    string
	Canonical      string
	OGImage        string
	OGType         string
	StructuredData template.HTML
}

type PageData struct {
	Metadata PageMetadata
	Content  any
}

func (a *Application) getBaseMetadata(rPath string) PageMetadata {
	return PageMetadata{
		Title:       "Orthodox Pilgrimage - Map of Saints' Relics in North America",
		Description: "Discover and venerate the relics of Orthodox Saints across North America. An interactive map of holy sites, monasteries, and cathedrals.",
		Canonical:   "https://orthodoxpilgrimage.com" + rPath,
		OGType:      "website",
	}
}

func (a *Application) getChurchMetadata(c sqlcdb.Church, relics []sqlcdb.ListRelicsForChurchRow) PageMetadata {
	var saintNames []string
	for _, r := range relics {
		saintNames = append(saintNames, r.Name)
	}

	title := fmt.Sprintf("%s - Orthodox Pilgrimage", c.Name)
	description := fmt.Sprintf("Venerate the relics of %s at %s in %s, %s.",
		strings.Join(saintNames, ", "), c.Name, c.City, c.StateProvince)
	if len(saintNames) == 0 {
		description = fmt.Sprintf("Visit %s in %s, %s for veneration and prayer.",
			c.Name, c.City, c.StateProvince)
	}

	if len(description) > 160 {
		description = description[:157] + "..."
	}

	canonical := fmt.Sprintf("https://orthodoxpilgrimage.com/churches/%s", c.Slug)

	ogImage := "https://orthodoxpilgrimage.com/static/og-image.jpg"
	if c.ImageUrl.Valid {
		ogImage = c.ImageUrl.String
	}

	return PageMetadata{
		Title:          title,
		Description:    description,
		Canonical:      canonical,
		OGImage:        ogImage,
		OGType:         "place",
		StructuredData: a.generateChurchJSONLD(c, relics),
	}
}

func (a *Application) getSaintMetadata(s sqlcdb.Saint) PageMetadata {
	title := fmt.Sprintf("%s - Orthodox Pilgrimage", s.Name)
	description := fmt.Sprintf("Find churches and holy sites housing the sacred relics of %s for veneration and pilgrimage.", s.Name)

	canonical := fmt.Sprintf("https://orthodoxpilgrimage.com/%s", s.Slug)

	ogImage := "https://orthodoxpilgrimage.com/static/og-image.jpg"
	if s.ImageUrl.Valid {
		ogImage = s.ImageUrl.String
	}

	return PageMetadata{
		Title:          title,
		Description:    description,
		Canonical:      canonical,
		OGImage:        ogImage,
		OGType:         "profile",
		StructuredData: a.generateSaintJSONLD(s),
	}
}

func (a *Application) generateChurchJSONLD(c sqlcdb.Church, relics []sqlcdb.ListRelicsForChurchRow) template.HTML {
	var saintNames []string
	for _, r := range relics {
		saintNames = append(saintNames, r.Name)
	}

	type ldAddress struct {
		Type            string `json:"@type"`
		StreetAddress   string `json:"streetAddress"`
		AddressLocality string `json:"addressLocality"`
		AddressRegion   string `json:"addressRegion"`
		AddressCountry  string `json:"addressCountry"`
	}

	type ldGeo struct {
		Type      string  `json:"@type"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}

	type ldPlace struct {
		Context     string    `json:"@context"`
		Type        string    `json:"@type"`
		Name        string    `json:"name"`
		Address     ldAddress `json:"address"`
		Geo         ldGeo     `json:"geo"`
		URL         string    `json:"url"`
		Description string    `json:"description"`
	}

	data := ldPlace{
		Context: "https://schema.org",
		Type:    "Place",
		Name:    c.Name,
		Address: ldAddress{
			Type:            "PostalAddress",
			StreetAddress:   c.AddressText,
			AddressLocality: c.City,
			AddressRegion:   c.StateProvince,
			AddressCountry:  c.CountryCode,
		},
		Geo: ldGeo{
			Type:      "GeoCoordinates",
			Latitude:  c.Latitude,
			Longitude: c.Longitude,
		},
		URL:         fmt.Sprintf("https://orthodoxpilgrimage.com/churches/%s", c.Slug),
		Description: fmt.Sprintf("Church housing the relics of: %s", strings.Join(saintNames, ", ")),
	}

	b, err := json.Marshal(data)
	if err != nil {
		slog.Error("Error marshaling JSON-LD", "error", err)
		return ""
	}

	return template.HTML(fmt.Sprintf("<script type=\"application/ld+json\">\n%s\n</script>", string(b)))
}

func (a *Application) generateSaintJSONLD(s sqlcdb.Saint) template.HTML {
	type ldPerson struct {
		Context     string `json:"@context"`
		Type        string `json:"@type"`
		Name        string `json:"name"`
		URL         string `json:"url"`
		Description string `json:"description"`
	}

	data := ldPerson{
		Context:     "https://schema.org",
		Type:        "Person",
		Name:        s.Name,
		URL:         fmt.Sprintf("https://orthodoxpilgrimage.com/%s", s.Slug),
		Description: s.Description.String,
	}

	b, err := json.Marshal(data)
	if err != nil {
		slog.Error("Error marshaling JSON-LD", "error", err)
		return ""
	}

	return template.HTML(fmt.Sprintf("<script type=\"application/ld+json\">\n%s\n</script>", string(b)))
}

func (a *Application) getChurchesDirectoryMetadata() PageMetadata {
	return PageMetadata{
		Title:       "Directory of Churches - Orthodox Pilgrimage",
		Description: "A complete list of Orthodox churches and monasteries housing sacred relics in North America.",
		Canonical:   "https://orthodoxpilgrimage.com/churches/",
		OGType:      "website",
	}
}

func (a *Application) getSaintsDirectoryMetadata() PageMetadata {
	return PageMetadata{
		Title:       "Directory of Saints - Orthodox Pilgrimage",
		Description: "Discover Orthodox Saints whose sacred relics are available for veneration across North America.",
		Canonical:   "https://orthodoxpilgrimage.com/saints/",
		OGType:      "website",
	}
}
