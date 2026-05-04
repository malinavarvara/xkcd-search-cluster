package db

import (
	"database/sql"
	"log/slog"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestDSN(t *testing.T) string {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}
	return dsn
}

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	dsn := getTestDSN(t)
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err, "failed to open database")
	require.NoError(t, db.Ping(), "failed to ping database")

	_, err = db.Exec(`DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)
	require.NoError(t, err, "failed to reset schema")

	cleanup := func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close database connection: %v", err)
		}
	}
	return db, cleanup
}

func TestMigrate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("success", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		err := Migrate(db, logger)
		require.NoError(t, err)

		var count int
		row := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'comics'`)
		err = row.Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "comics table should exist")
	})

	t.Run("no changes", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		err := Migrate(db, logger)
		require.NoError(t, err)

		err = Migrate(db, logger)
		assert.NoError(t, err, "second migration should not error")
	})

	t.Run("nil db", func(t *testing.T) {
		err := Migrate(nil, logger)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "database connection is nil")
	})

	t.Run("invalid db", func(t *testing.T) {
		db, err := sql.Open("pgx", "postgres://invalid:invalid@localhost:1/none")
		require.NoError(t, err)
		err = Migrate(db, logger)
		assert.Error(t, err, "should fail on invalid database connection")
	})
}
