package app

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/edwlarkey/orthodoxpilgrimage/internal/db"
	sqlcdb "github.com/edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestValidateDataJSON(t *testing.T) {
	f, err := os.Open("data/data.json")
	require.NoError(t, err)
	defer f.Close()

	dsn := "file:valdb?mode=memory&cache=shared"
	dbConn, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	defer dbConn.Close()

	err = db.MigrateUp(dbConn)
	require.NoError(t, err)

	queries := sqlcdb.New(dbConn)
	err = SeedFromReader(context.Background(), queries, f)
	require.NoError(t, err, "data.json validation failed")
}
