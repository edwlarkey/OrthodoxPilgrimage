package app

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	sqlcdb "github.com/edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
)

//go:embed data/data.json
var dataFile embed.FS

type DataFile struct {
	Saints   []SaintData  `json:"saints"`
	Churches []ChurchData `json:"churches"`
}

type ImageData struct {
	URL       string `json:"url"`
	AltText   string `json:"alt_text"`
	Source    string `json:"source"`
	IsPrimary bool   `json:"is_primary"`
	SortOrder int32  `json:"sort_order"`
}

type SaintData struct {
	Name        string      `json:"name"`
	Slug        string      `json:"slug"`
	FeastDay    string      `json:"feast_day"`
	Description string      `json:"description"`
	Images      []ImageData `json:"images"`
	LivesURL    string      `json:"lives_url"`
	UpdatedAt   string      `json:"updated_at"`
}

type ChurchData struct {
	Name          string      `json:"name"`
	Slug          string      `json:"slug"`
	Type          string      `json:"type"`
	AddressText   string      `json:"address_text"`
	City          string      `json:"city"`
	StateProvince string      `json:"state_province"`
	PostalCode    string      `json:"postal_code"`
	CountryCode   string      `json:"country_code"`
	Latitude      float64     `json:"latitude"`
	Longitude     float64     `json:"longitude"`
	Jurisdiction  string      `json:"jurisdiction"`
	Website       string      `json:"website"`
	Phone         string      `json:"phone"`
	Description   string      `json:"description"`
	Images        []ImageData `json:"images"`
	Relics        []RelicData `json:"relics"`
	Sources       []string    `json:"sources"`
	UpdatedAt     string      `json:"updated_at"`
}

type RelicData struct {
	SaintSlug   string      `json:"saint_slug"`
	Description string      `json:"description"`
	Images      []ImageData `json:"images"`
}

func SeedDatabase(ctx context.Context, queries *sqlcdb.Queries) error {
	start := time.Now()
	data, err := dataFile.ReadFile("data/data.json")
	if err != nil {
		return fmt.Errorf("failed to read embedded data file: %w", err)
	}
	err = SeedFromReader(ctx, queries, bytes.NewReader(data))
	if err == nil {
		slog.Info("Database seeding complete", "duration", time.Since(start))
	}
	return err
}

func SeedFromReader(ctx context.Context, queries *sqlcdb.Queries, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	var df DataFile
	if err := json.Unmarshal(data, &df); err != nil {
		return fmt.Errorf("failed to parse data: %w", err)
	}

	// 2. Clear existing data
	if err := queries.DeleteAllImages(ctx); err != nil {
		return fmt.Errorf("failed to delete images: %w", err)
	}
	if err := queries.DeleteAllRelics(ctx); err != nil {
		return fmt.Errorf("failed to delete relics: %w", err)
	}
	if err := queries.DeleteAllSources(ctx); err != nil {
		return fmt.Errorf("failed to delete sources: %w", err)
	}
	if err := queries.DeleteAllSaints(ctx); err != nil {
		return fmt.Errorf("failed to delete saints: %w", err)
	}
	if err := queries.DeleteAllChurches(ctx); err != nil {
		return fmt.Errorf("failed to delete churches: %w", err)
	}

	// 3. Insert Saints
	saintMap := make(map[string]int64)
	for _, s := range df.Saints {
		updatedAt := sql.NullTime{}
		if s.UpdatedAt != "" {
			if t, err := time.Parse("2006-01-02", s.UpdatedAt); err == nil {
				updatedAt = sql.NullTime{Time: t, Valid: true}
			}
		}
		saint, err := queries.CreateSaint(ctx, sqlcdb.CreateSaintParams{
			Name: s.Name,
			Slug: s.Slug,
			FeastDay: sql.NullString{
				String: s.FeastDay,
				Valid:  s.FeastDay != "",
			},
			Description: sql.NullString{
				String: s.Description,
				Valid:  s.Description != "",
			},
			LivesUrl: sql.NullString{
				String: s.LivesURL,
				Valid:  s.LivesURL != "",
			},
			Status:    "published",
			UpdatedAt: updatedAt,
		})
		if err != nil {
			return fmt.Errorf("failed to create saint %s: %w", s.Name, err)
		}
		saintMap[s.Slug] = saint.ID

		for _, img := range s.Images {
			err = queries.CreateImage(ctx, sqlcdb.CreateImageParams{
				SaintID: sql.NullInt64{Int64: saint.ID, Valid: true},
				Url:     img.URL,
				AltText: sql.NullString{String: img.AltText, Valid: img.AltText != ""},
				Source:  sql.NullString{String: img.Source, Valid: img.Source != ""},
				IsPrimary: sql.NullBool{
					Bool:  img.IsPrimary,
					Valid: true,
				},
				SortOrder: sql.NullInt64{
					Int64: int64(img.SortOrder),
					Valid: true,
				},
			})
			if err != nil {
				return fmt.Errorf("failed to create image for saint %s: %w", s.Name, err)
			}
		}
	}

	// 3.5 Load or create Jurisdictions
	jurisdictionMap := make(map[string]int64)

	// 4. Insert Churches
	for _, c := range df.Churches {
		updatedAt := sql.NullTime{}
		if c.UpdatedAt != "" {
			if t, err := time.Parse("2006-01-02", c.UpdatedAt); err == nil {
				updatedAt = sql.NullTime{Time: t, Valid: true}
			}
		}

		var jurisdictionID sql.NullInt64
		if c.Jurisdiction != "" {
			if jID, ok := jurisdictionMap[c.Jurisdiction]; ok {
				jurisdictionID = sql.NullInt64{Int64: jID, Valid: true}
			} else {
				tradition := "Orthodox"
				pinColor := "#530c38"
				if strings.Contains(c.Jurisdiction, "Roman Catholic") {
					tradition = "Roman Catholic"
					pinColor = "#2e5a88"
				}
				j, err := queries.CreateJurisdiction(ctx, sqlcdb.CreateJurisdictionParams{
					Name:      c.Jurisdiction,
					Tradition: tradition,
					PinColor:  pinColor,
				})
				if err == nil {
					jurisdictionMap[c.Jurisdiction] = j.ID
					jurisdictionID = sql.NullInt64{Int64: j.ID, Valid: true}
				}
			}
		}

		church, err := queries.CreateChurch(ctx, sqlcdb.CreateChurchParams{
			Name:           c.Name,
			Slug:           c.Slug,
			Type:           sql.NullString{String: c.Type, Valid: c.Type != ""},
			AddressText:    c.AddressText,
			City:           c.City,
			StateProvince:  c.StateProvince,
			PostalCode:     sql.NullString{String: c.PostalCode, Valid: c.PostalCode != ""},
			CountryCode:    c.CountryCode,
			Latitude:       c.Latitude,
			Longitude:      c.Longitude,
			JurisdictionID: jurisdictionID,
			Website:        sql.NullString{String: c.Website, Valid: c.Website != ""},
			Phone:          sql.NullString{String: c.Phone, Valid: c.Phone != ""},
			Description:    sql.NullString{String: c.Description, Valid: c.Description != ""},
			Status:         "published",
			UpdatedAt:      updatedAt,
		})
		if err != nil {
			return fmt.Errorf("failed to create church %s: %w", c.Name, err)
		}

		for _, img := range c.Images {
			err = queries.CreateImage(ctx, sqlcdb.CreateImageParams{
				ChurchID: sql.NullInt64{Int64: church.ID, Valid: true},
				Url:      img.URL,
				AltText:  sql.NullString{String: img.AltText, Valid: img.AltText != ""},
				Source:   sql.NullString{String: img.Source, Valid: img.Source != ""},
				IsPrimary: sql.NullBool{
					Bool:  img.IsPrimary,
					Valid: true,
				},
				SortOrder: sql.NullInt64{
					Int64: int64(img.SortOrder),
					Valid: true,
				},
			})
			if err != nil {
				return fmt.Errorf("failed to create image for church %s: %w", c.Name, err)
			}
		}

		for _, r := range c.Relics {
			saintID, ok := saintMap[r.SaintSlug]
			if !ok {
				return fmt.Errorf("church %s references unknown saint slug %s", c.Name, r.SaintSlug)
			}

			err = queries.CreateRelic(ctx, sqlcdb.CreateRelicParams{
				ChurchID:    church.ID,
				SaintID:     saintID,
				RelicTypeID: sql.NullInt64{Int64: 2, Valid: true}, // Default to Fragment
				Description: sql.NullString{
					String: r.Description,
					Valid:  r.Description != "",
				},
			})
			if err != nil {
				return fmt.Errorf("failed to create relic for church %s: %w", c.Name, err)
			}

			for _, img := range r.Images {
				err = queries.CreateImage(ctx, sqlcdb.CreateImageParams{
					RelicChurchID: sql.NullInt64{Int64: church.ID, Valid: true},
					RelicSaintID:  sql.NullInt64{Int64: saintID, Valid: true},
					Url:           img.URL,
					AltText:       sql.NullString{String: img.AltText, Valid: img.AltText != ""},
					Source:        sql.NullString{String: img.Source, Valid: img.Source != ""},
					IsPrimary: sql.NullBool{
						Bool:  img.IsPrimary,
						Valid: true,
					},
					SortOrder: sql.NullInt64{
						Int64: int64(img.SortOrder),
						Valid: true,
					},
				})
				if err != nil {
					return fmt.Errorf("failed to create image for relic of %s in %s: %w", r.SaintSlug, c.Name, err)
				}
			}
		}

		// Insert sources for this church
		for _, source := range c.Sources {
			err = queries.CreateChurchSource(ctx, sqlcdb.CreateChurchSourceParams{
				ChurchID: church.ID,
				Source:   source,
			})
			if err != nil {
				return fmt.Errorf("failed to create source for church %s: %w", c.Name, err)
			}
		}
	}

	return nil
}
