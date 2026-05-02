package db

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"yadro.com/course/search/core"
)

type repository struct {
	db     *sqlx.DB
	logger *slog.Logger
}

type dbComic struct {
	Num    int    `db:"num"`
	ImgURL string `db:"img_url"`
}

func NewRepository(db *sqlx.DB, logger *slog.Logger) core.ComicsRepository {
	return &repository{
		db:     db,
		logger: logger,
	}
}

func (r *repository) SearchComics(ctx context.Context, words []string, limit int) ([]core.Comics, error) {

	if len(words) == 0 {
		return []core.Comics{}, nil
	}

	query := `
        SELECT 
            c.num, 
            c.img_url
        FROM comics c
        JOIN comic_words cw ON c.id = cw.comic_id
        JOIN words w ON cw.word_id = w.id
        WHERE w.word = ANY($1) 
        GROUP BY c.id, c.num, c.img_url
        ORDER BY COUNT(DISTINCT w.id) DESC, c.num DESC
        LIMIT $2
    `

	var rows []dbComic
	err := r.db.SelectContext(ctx, &rows, query, pq.Array(words), limit)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}

	result := make([]core.Comics, 0, len(rows))
	for _, row := range rows {
		result = append(result, core.Comics{
			Num:    row.Num,
			ImgURL: row.ImgURL,
		})
	}

	return result, nil
}

func (r *repository) CountSearchResults(ctx context.Context, words []string) (int, error) {
	if len(words) == 0 {
		return 0, nil
	}

	query := `
		SELECT COUNT(DISTINCT c.id)
		FROM comics c
		JOIN comic_words cw ON c.id = cw.comic_id
		JOIN words w ON cw.word_id = w.id
		WHERE w.word = ANY($1)
	`

	var total int
	err := r.db.GetContext(ctx, &total, query, pq.Array(words))
	if err != nil {
		return 0, fmt.Errorf("failed to count search results: %w", err)
	}
	return total, nil
}

func (r *repository) GetAllComicsWords(ctx context.Context) (map[string][]int, error) {
	query := `
        SELECT w.word, c.num 
        FROM words w
        JOIN comic_words cw ON w.id = cw.word_id
        JOIN comics c ON c.id = cw.comic_id
    `

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch all words: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			r.logger.Error("failed to close rows", "error", closeErr)
		}
	}()

	index := make(map[string][]int)
	for rows.Next() {
		var word string
		var num int
		if err := rows.Scan(&word, &num); err != nil {
			return nil, err
		}
		index[word] = append(index[word], num)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return index, nil
}

func (r *repository) GetComicsByIDs(ctx context.Context, ids []int) ([]core.Comics, error) {
	if len(ids) == 0 {
		return []core.Comics{}, nil
	}

	query := `
		SELECT num, img_url 
		FROM comics 
		WHERE num = ANY($1)
	`

	var rows []dbComic
	err := r.db.SelectContext(ctx, &rows, query, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("failed to get comics by ids: %w", err)
	}

	result := make([]core.Comics, 0, len(rows))
	for _, row := range rows {
		result = append(result, core.Comics{
			Num:    row.Num,
			ImgURL: row.ImgURL,
		})
	}

	return result, nil
}
