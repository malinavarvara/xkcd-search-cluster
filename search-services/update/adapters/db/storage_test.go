package db

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"yadro.com/course/update/core"
)

func setupStorage(t *testing.T) (*DB, func()) {
	dsn := getTestDSN(t)
	dbSQL, err := sql.Open("pgx", dsn)
	require.NoError(t, err, "failed to open database")
	require.NoError(t, dbSQL.Ping(), "database ping failed")

	_, err = dbSQL.Exec(`DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)
	require.NoError(t, err, "failed to reset schema")

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	err = Migrate(dbSQL, logger)
	require.NoError(t, err, "migration failed")

	db := &DB{
		log:  logger,
		conn: sqlx.NewDb(dbSQL, "pgx"),
	}

	cleanup := func() {
		if err := dbSQL.Close(); err != nil {
			t.Logf("failed to close database connection: %v", err)
		}
	}
	return db, cleanup
}

func TestNewDB(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	_, err := New(logger, "postgres://invalid:invalid@localhost:1/")
	assert.Error(t, err, "expected error for invalid address")
}

func TestNewDB_Success(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	db, err := New(logger, dsn)
	require.NoError(t, err, "failed to create DB")
	require.NotNil(t, db)
	err = db.Close()
	require.NoError(t, err, "failed to close DB")
}

func TestDB_Migrate(t *testing.T) {
	db, cleanup := setupStorage(t)
	defer cleanup()
	err := db.Migrate()
	assert.NoError(t, err, "migrate should succeed on clean schema")
}

func TestDB_Add(t *testing.T) {
	db, cleanup := setupStorage(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("insert new comic", func(t *testing.T) {
		comic := core.Comics{
			Num:         1,
			ImgURL:      "https://example.com/1.png",
			PublishedAt: time.Now().UTC(),
			Words:       []string{"hello", "world"},
		}
		err := db.Add(ctx, comic)
		require.NoError(t, err)

		var count int
		err = db.conn.GetContext(ctx, &count, "SELECT COUNT(*) FROM comics WHERE num = $1", 1)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "comic not inserted")

		var wordCount int
		err = db.conn.GetContext(ctx, &wordCount,
			`SELECT COUNT(*) FROM comic_words cw 
			 JOIN words w ON w.id = cw.word_id 
			 WHERE w.word IN ('hello', 'world')`)
		require.NoError(t, err)
		assert.Equal(t, 2, wordCount, "word relations count mismatch")
	})
}

func TestDB_Stats(t *testing.T) {
	db, cleanup := setupStorage(t)
	defer cleanup()
	ctx := context.Background()

	stats, err := db.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.WordsTotal)
	assert.Equal(t, 0, stats.WordsUnique)
	assert.Equal(t, 0, stats.ComicsFetched)

	comic1 := core.Comics{Num: 10, ImgURL: "url1", PublishedAt: time.Now(), Words: []string{"a", "b", "c"}}
	comic2 := core.Comics{Num: 20, ImgURL: "url2", PublishedAt: time.Now(), Words: []string{"a", "d"}}
	require.NoError(t, db.Add(ctx, comic1))
	require.NoError(t, db.Add(ctx, comic2))

	stats, err = db.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 5, stats.WordsTotal, "total word occurrences")
	assert.Equal(t, 4, stats.WordsUnique, "unique words")
	assert.Equal(t, 2, stats.ComicsFetched, "comics count")
}

func TestDB_IDs(t *testing.T) {
	db, cleanup := setupStorage(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("empty set", func(t *testing.T) {
		ids, err := db.IDs(ctx)
		require.NoError(t, err)
		assert.Empty(t, ids)
	})

	t.Run("with data", func(t *testing.T) {
		nums := []int{100, 200, 50}
		for _, num := range nums {
			comic := core.Comics{Num: num, ImgURL: "url", PublishedAt: time.Now()}
			require.NoError(t, db.Add(ctx, comic))
		}
		ids, err := db.IDs(ctx)
		require.NoError(t, err)
		assert.ElementsMatch(t, []int{50, 100, 200}, ids)
	})
}

func TestDB_Drop(t *testing.T) {
	db, cleanup := setupStorage(t)
	defer cleanup()
	ctx := context.Background()

	comic := core.Comics{Num: 999, ImgURL: "url", PublishedAt: time.Now(), Words: []string{"test"}}
	require.NoError(t, db.Add(ctx, comic))

	err := db.Drop(ctx)
	require.NoError(t, err)

	var count int
	err = db.conn.GetContext(ctx, &count, "SELECT COUNT(*) FROM comics")
	require.NoError(t, err)
	assert.Equal(t, 0, count, "comics table not empty")

	err = db.conn.GetContext(ctx, &count, "SELECT COUNT(*) FROM words")
	require.NoError(t, err)
	assert.Equal(t, 0, count, "words table not empty")

	err = db.conn.GetContext(ctx, &count, "SELECT COUNT(*) FROM comic_words")
	require.NoError(t, err)
	assert.Equal(t, 0, count, "comic_words table not empty")

	var seqVal int
	err = db.conn.GetContext(ctx, &seqVal, "SELECT nextval('comics_id_seq')")
	require.NoError(t, err)
	assert.Equal(t, 1, seqVal, "sequence not reset")
}

func TestDB_Close(t *testing.T) {
	db, cleanup := setupStorage(t)
	defer cleanup()

	err := db.Close()
	require.NoError(t, err)

	_, err = db.conn.Exec("SELECT 1")
	assert.Error(t, err, "expected error on closed connection")
}
