package app

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"time"

	sqlcdb "github.com/edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
)

//go:embed data/data.json
var dataFile embed.FS

type DataFile struct {
	Saints   []SaintData  `json:"saints"`
	Churches []ChurchData `json:"churches"`
}

type SaintData struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	FeastDay    string `json:"feast_day"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	LivesURL    string `json:"lives_url"`
	UpdatedAt   string `json:"updated_at"`
}

type ChurchData struct {
	Name          string      `json:"name"`
	Slug          string      `json:"slug"`
	Type          string      `json:"type"`
	AddressText   string      `json:"address_text"`
	City          string      `json:"city"`
	StateProvince string      `json:"state_province"`
	CountryCode   string      `json:"country_code"`
	Latitude      float64     `json:"latitude"`
	Longitude     float64     `json:"longitude"`
	Jurisdiction  string      `json:"jurisdiction"`
	Website       string      `json:"website"`
	Phone         string      `json:"phone"`
	Description   string      `json:"description"`
	ImageURL      string      `json:"image_url"`
	Relics        []RelicData `json:"relics"`
	Sources       []string    `json:"sources"`
	UpdatedAt     string      `json:"updated_at"`
}

type RelicData struct {
	SaintSlug   string `json:"saint_slug"`
	Description string `json:"description"`
}

func SeedDatabase(ctx context.Context, queries *sqlcdb.Queries) error {
	data, err := dataFile.ReadFile("data/data.json")
	if err != nil {
		return fmt.Errorf("failed to read embedded data file: %w", err)
	}
	return SeedFromReader(ctx, queries, bytes.NewReader(data))
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
			ImageUrl: sql.NullString{
				String: s.ImageURL,
				Valid:  s.ImageURL != "",
			},
			LivesUrl: sql.NullString{
				String: s.LivesURL,
				Valid:  s.LivesURL != "",
			},
			UpdatedAt: updatedAt,
		})
		if err != nil {
			return fmt.Errorf("failed to create saint %s: %w", s.Name, err)
		}
		saintMap[s.Slug] = saint.ID
	}

	// 4. Insert Churches and Relics
	for _, c := range df.Churches {
		updatedAt := sql.NullTime{}
		if c.UpdatedAt != "" {
			if t, err := time.Parse("2006-01-02", c.UpdatedAt); err == nil {
				updatedAt = sql.NullTime{Time: t, Valid: true}
			}
		}
		church, err := queries.CreateChurch(ctx, sqlcdb.CreateChurchParams{
			Name:          c.Name,
			Slug:          c.Slug,
			Type:          sql.NullString{String: c.Type, Valid: c.Type != ""},
			AddressText:   c.AddressText,
			City:          c.City,
			StateProvince: c.StateProvince,
			CountryCode:   c.CountryCode,
			Latitude:      c.Latitude,
			Longitude:     c.Longitude,
			Jurisdiction:  sql.NullString{String: c.Jurisdiction, Valid: c.Jurisdiction != ""},
			Website:       sql.NullString{String: c.Website, Valid: c.Website != ""},
			Phone:         sql.NullString{String: c.Phone, Valid: c.Phone != ""},
			Description:   sql.NullString{String: c.Description, Valid: c.Description != ""},
			ImageUrl:      sql.NullString{String: c.ImageURL, Valid: c.ImageURL != ""},
			UpdatedAt:     updatedAt,
		})
		if err != nil {
			return fmt.Errorf("failed to create church %s: %w", c.Name, err)
		}

		for _, r := range c.Relics {
			saintID, ok := saintMap[r.SaintSlug]
			if !ok {
				return fmt.Errorf("church %s references unknown saint slug %s", c.Name, r.SaintSlug)
			}

			err = queries.CreateRelic(ctx, sqlcdb.CreateRelicParams{
				ChurchID: church.ID,
				SaintID:  saintID,
				Description: sql.NullString{
					String: r.Description,
					Valid:  r.Description != "",
				},
			})
			if err != nil {
				return fmt.Errorf("failed to create relic for church %s: %w", c.Name, err)
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
