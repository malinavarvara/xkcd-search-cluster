package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"yadro.com/course/update/core"
)

type DB struct {
	log  *slog.Logger
	conn *sqlx.DB
}

func New(log *slog.Logger, address string) (*DB, error) {

	db, err := sqlx.Connect("pgx", address)
	if err != nil {
		log.Error("connection problem", "address", address, "error", err)
		return nil, err
	}

	return &DB{
		log:  log,
		conn: db,
	}, nil
}

func (db *DB) Migrate() error {
	return Migrate(db.conn.DB, db.log)
}

func (db *DB) Add(ctx context.Context, comics core.Comics) error {
	const maxRetries = 5
	var err error

	for i := range maxRetries {
		err = db.addOnce(ctx, comics)
		if err == nil {
			return nil
		}

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.DeadlockDetected {
			db.log.Warn("deadlock detected, retrying",
				"num", comics.Num,
				"attempt", i+1,
			)
			time.Sleep(time.Duration(10*(i+1)) * time.Millisecond)
			continue
		}
		return err
	}
	return fmt.Errorf("failed after %d retries: %w", maxRetries, err)
}

func (db *DB) addOnce(ctx context.Context, comics core.Comics) error {
	tx, err := db.conn.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		rbErr := tx.Rollback()
		if rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			db.log.Error("failed to rollback transaction", "error", rbErr)
		}
	}()

	var comicID int
	comicQuery := `
		INSERT INTO comics (num, img_url, published_at) 
		VALUES ($1, $2, $3) 
		ON CONFLICT (num) DO UPDATE SET 
			img_url = EXCLUDED.img_url, 
			published_at = EXCLUDED.published_at
		RETURNING id`

	if err := tx.QueryRowxContext(ctx, comicQuery,
		comics.Num, comics.ImgURL, comics.PublishedAt,
	).Scan(&comicID); err != nil {
		return fmt.Errorf("upsert comic: %w", err)
	}

	if len(comics.Words) == 0 {
		return tx.Commit()
	}

	wordMap := make(map[string]struct{})
	for _, w := range comics.Words {
		if w = strings.TrimSpace(w); w != "" {
			wordMap[w] = struct{}{}
		}
	}
	uniqueWords := make([]string, 0, len(wordMap))
	for w := range wordMap {
		uniqueWords = append(uniqueWords, w)
	}
	sort.Strings(uniqueWords)

	wordsQuery := `
		INSERT INTO words (word) 
		SELECT * FROM unnest($1::text[])
		ON CONFLICT (word) DO NOTHING`

	if _, err := tx.ExecContext(ctx, wordsQuery, uniqueWords); err != nil {
		return fmt.Errorf("bulk insert words: %w", err)
	}

	var wordIDs []int
	selectQuery := `SELECT id FROM words WHERE word = ANY($1::text[])`
	if err := tx.SelectContext(ctx, &wordIDs, selectQuery, uniqueWords); err != nil {
		return fmt.Errorf("get word ids: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM comic_words WHERE comic_id = $1`, comicID); err != nil {
		return fmt.Errorf("delete old relations: %w", err)
	}

	if len(wordIDs) > 0 {
		relQuery := `
			INSERT INTO comic_words (comic_id, word_id)
			SELECT $1, unnest($2::int[])
			ON CONFLICT DO NOTHING`

		if _, err := tx.ExecContext(ctx, relQuery, comicID, wordIDs); err != nil {
			return fmt.Errorf("bulk insert relations: %w", err)
		}
	}

	return tx.Commit()
}

func (db *DB) Stats(ctx context.Context) (core.DBStats, error) {
	var stats core.DBStats
	query := `
        SELECT 
            (SELECT COUNT(*) FROM comic_words),
            (SELECT COUNT(*) FROM words),
            (SELECT COUNT(*) FROM comics)`

	err := db.conn.QueryRowContext(ctx, query).Scan(
		&stats.WordsTotal,
		&stats.WordsUnique,
		&stats.ComicsFetched,
	)
	if err != nil {
		return core.DBStats{}, fmt.Errorf("get stats: %w", err)
	}

	return stats, nil
}

func (db *DB) IDs(ctx context.Context) ([]int, error) {
	var nums []int
	err := db.conn.SelectContext(ctx, &nums, `SELECT num FROM comics ORDER BY num`)
	if err != nil {
		return nil, fmt.Errorf("select nums: %w", err)
	}
	return nums, nil
}

func (db *DB) Drop(ctx context.Context) error {
	query := `TRUNCATE comics, words, comic_words RESTART IDENTITY CASCADE`
	_, err := db.conn.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("truncate tables: %w", err)
	}
	return nil
}

func (db *DB) Close() error {
	db.log.Debug("closing database connection")
	return db.conn.Close()
}
