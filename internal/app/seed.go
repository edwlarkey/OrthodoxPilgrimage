package app

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io"

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
	if err := queries.DeleteAllSaints(ctx); err != nil {
		return fmt.Errorf("failed to delete saints: %w", err)
	}
	if err := queries.DeleteAllChurches(ctx); err != nil {
		return fmt.Errorf("failed to delete churches: %w", err)
	}

	// 3. Insert Saints
	saintMap := make(map[string]int64)
	for _, s := range df.Saints {
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
		})
		if err != nil {
			return fmt.Errorf("failed to create saint %s: %w", s.Name, err)
		}
		saintMap[s.Slug] = saint.ID
	}

	// 4. Insert Churches and Relics
	for _, c := range df.Churches {
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
	}

	return nil
}
