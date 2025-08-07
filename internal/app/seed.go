package app

import (
	"context"
	"database/sql"
	"fmt"

	sqlcdb "git.sr.ht/~edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
)

func SeedDatabase(ctx context.Context, queries *sqlcdb.Queries) error {
	churches := []sqlcdb.CreateChurchParams{
		{
			Name:          "St. John the Baptist Greek Orthodox Church",
			AddressText:   "123 Main St, New York, NY 10001",
			City:          "New York",
			StateProvince: "NY",
			CountryCode:   "US",
			Latitude:      40.7128,
			Longitude:     -74.0060,
			Jurisdiction: sql.NullString{
				String: "Greek Orthodox Archdiocese of America",
				Valid:  true,
			},
		},
		{
			Name:          "Holy Trinity Orthodox Cathedral",
			AddressText:   "1121 N Leavitt St, Chicago, IL 60622",
			City:          "Chicago",
			StateProvince: "IL",
			CountryCode:   "US",
			Latitude:      41.9022,
			Longitude:     -87.6818,
			Jurisdiction: sql.NullString{
				String: "Orthodox Church in America",
				Valid:  true,
			},
		},
	}

	for _, church := range churches {
		_, err := queries.CreateChurch(ctx, church)
		if err != nil {
			return fmt.Errorf("failed to create church %s: %w", church.Name, err)
		}
	}
	return nil
}
