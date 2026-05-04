package db

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_SearchComics(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close db: %v", err)
		}
	}()

	sqlxDB := sqlx.NewDb(db, "postgres")
	logger := slog.Default()
	repo := NewRepository(sqlxDB, logger)

	t.Run("Success", func(t *testing.T) {
		words := []string{"apple", "banana"}
		limit := 2

		rows := sqlmock.NewRows([]string{"num", "img_url"}).
			AddRow(1, "https://xkcd.com/1/info.0.json").
			AddRow(2, "https://xkcd.com/2/info.0.json")

		mock.ExpectQuery(`SELECT c.num, c.img_url FROM comics c`).
			WithArgs(sqlmock.AnyArg(), limit).
			WillReturnRows(rows)

		result, err := repo.SearchComics(context.Background(), words, limit)
		assert.NoError(t, err)
		assert.Len(t, result, 2, "expected 2 comics")
		assert.Equal(t, 1, result[0].Num, "first comic number mismatch")
		assert.Equal(t, "https://xkcd.com/1/info.0.json", result[0].ImgURL, "first comic URL mismatch")
	})

	t.Run("EmptyWords", func(t *testing.T) {
		result, err := repo.SearchComics(context.Background(), []string{}, 10)
		assert.NoError(t, err)
		assert.Empty(t, result, "expected empty result for empty words")
	})
}

func TestRepository_CountSearchResults(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close db: %v", err)
		}
	}()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewRepository(sqlxDB, slog.Default())

	t.Run("Success", func(t *testing.T) {
		words := []string{"test"}

		mock.ExpectQuery(`SELECT COUNT\(DISTINCT c.id\)`).
			WithArgs(sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))

		total, err := repo.CountSearchResults(context.Background(), words)
		assert.NoError(t, err)
		assert.Equal(t, 42, total, "count mismatch")
	})
}

func TestRepository_GetAllComicsWords(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close db: %v", err)
		}
	}()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewRepository(sqlxDB, slog.Default())

	t.Run("Success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"word", "num"}).
			AddRow("hero", 1).
			AddRow("hero", 2).
			AddRow("world", 1)

		mock.ExpectQuery(`SELECT w.word, c.num FROM words w`).
			WillReturnRows(rows)

		index, err := repo.GetAllComicsWords(context.Background())
		assert.NoError(t, err)
		assert.Len(t, index, 2, "expected 2 words in index")
		assert.ElementsMatch(t, []int{1, 2}, index["hero"], "hero comic IDs mismatch")
		assert.Equal(t, []int{1}, index["world"], "world comic IDs mismatch")
	})
}

func TestRepository_GetComicsByIDs(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close db: %v", err)
		}
	}()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewRepository(sqlxDB, slog.Default())

	t.Run("Success", func(t *testing.T) {
		ids := []int{10, 20}

		rows := sqlmock.NewRows([]string{"num", "img_url"}).
			AddRow(10, "url10").
			AddRow(20, "url20")

		mock.ExpectQuery(`SELECT num, img_url FROM comics WHERE num = ANY`).
			WithArgs(sqlmock.AnyArg()).
			WillReturnRows(rows)

		result, err := repo.GetComicsByIDs(context.Background(), ids)
		assert.NoError(t, err)
		assert.Len(t, result, 2, "expected 2 comics")
		assert.Equal(t, 10, result[0].Num, "first comic ID mismatch")
	})
}

func TestRepository_SearchComics_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close db: %v", err)
		}
	}()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewRepository(sqlxDB, slog.Default())

	words := []string{"apple"}
	limit := 5

	mock.ExpectQuery(`SELECT c.num, c.img_url FROM comics c`).
		WithArgs(sqlmock.AnyArg(), limit).
		WillReturnError(errors.New("database error"))

	_, err = repo.SearchComics(context.Background(), words, limit)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "search query failed")
}

func TestRepository_CountSearchResults_EmptyWords(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close db: %v", err)
		}
	}()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewRepository(sqlxDB, slog.Default())

	total, err := repo.CountSearchResults(context.Background(), []string{})
	assert.NoError(t, err)
	assert.Equal(t, 0, total, "expected zero total for empty words")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_CountSearchResults_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close db: %v", err)
		}
	}()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewRepository(sqlxDB, slog.Default())

	words := []string{"test"}
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT c.id\)`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnError(errors.New("count error"))

	_, err = repo.CountSearchResults(context.Background(), words)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to count search results")
}

func TestRepository_GetAllComicsWords_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close db: %v", err)
		}
	}()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewRepository(sqlxDB, slog.Default())

	mock.ExpectQuery(`SELECT w.word, c.num FROM words w`).
		WillReturnError(errors.New("query error"))

	_, err = repo.GetAllComicsWords(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch all words")
}

func TestRepository_GetAllComicsWords_ScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close db: %v", err)
		}
	}()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewRepository(sqlxDB, slog.Default())

	rows := sqlmock.NewRows([]string{"word"}).AddRow("hero")
	mock.ExpectQuery(`SELECT w.word, c.num FROM words w`).WillReturnRows(rows)

	_, err = repo.GetAllComicsWords(context.Background())
	assert.Error(t, err, "expected scan error")
}

func TestRepository_GetComicsByIDs_EmptyIDs(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close db: %v", err)
		}
	}()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewRepository(sqlxDB, slog.Default())

	result, err := repo.GetComicsByIDs(context.Background(), []int{})
	assert.NoError(t, err)
	assert.Empty(t, result, "expected empty result for empty IDs")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetComicsByIDs_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("failed to close db: %v", err)
		}
	}()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := NewRepository(sqlxDB, slog.Default())

	ids := []int{10, 20}
	mock.ExpectQuery(`SELECT num, img_url FROM comics WHERE num = ANY`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnError(errors.New("select error"))

	_, err = repo.GetComicsByIDs(context.Background(), ids)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get comics by ids")
}
