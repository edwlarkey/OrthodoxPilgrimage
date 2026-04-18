package db

import (
	"database/sql"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateUp(t *testing.T) {
	// Use an in-memory SQLite database for testing.
	dsn := "file:memdb?mode=memory&cache=shared"
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err, "failed to connect to in-memory db")
	defer db.Close()

	// 1. Run migrations for the first time.
	err = MigrateUp(db)
	require.NoError(t, err, "MigrateUp should succeed on the first run")

	// 2. Verify that the 'churches' table was created.
	_, err = db.Exec("SELECT id FROM churches LIMIT 1;")
	assert.NoError(t, err, "churches table should exist after migration")

	// 3. Verify that the 'schema_migrations' table was created and has entries.
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	require.NoError(t, err)
	assert.Greater(t, count, 0, "schema_migrations table should have entries")

	// 4. Run migrations for the second time to test idempotency.
	err = MigrateUp(db)
	require.NoError(t, err, "MigrateUp should succeed on the second run (be idempotent)")
}

// TestMain allows us to set up and tear down the database for all tests in this package.
// For now, we are creating a fresh DB for each test, but this is a good pattern to know.
func TestMain(m *testing.M) {
	// You could do setup here, like creating a test DB file.
	// For now, we don't need to.
	code := m.Run()
	// You could do teardown here, like deleting the test DB file.
	os.Exit(code)
}
