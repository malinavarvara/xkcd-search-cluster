package db

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func Migrate(db *sql.DB, logger *slog.Logger) error {
	logger.Debug("applying database migrations")
	if db == nil {
		return errors.New("database connection is nil")
	}

	src, err := iofs.New(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("create iofs source: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create postgres driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}

	err = m.Up()
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.Debug("no migrations to apply")
			return nil
		}
		logger.Error("migration failed", "error", err)
		return fmt.Errorf("apply migrations: %w", err)
	}

	logger.Debug("migrations applied successfully")
	return nil
}
