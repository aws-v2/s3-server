package database

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var MigrationFiles embed.FS

// RunMigrations executes all pending migrations
func RunMigrations(db *sql.DB, dbName string) error {
	sourceDriver, err := iofs.New(MigrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	databaseDriver, err := postgres.WithInstance(db, &postgres.Config{
		DatabaseName: dbName,
	})
	if err != nil {
		return fmt.Errorf("failed to create database driver: %w", err)
	}

	m, err := migrate.NewWithInstance(
		"iofs",
		sourceDriver,
		dbName,
		databaseDriver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	slog.Info("[MIGRATE] Running up migrations")

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// ForceVersion forces the migration version and clears dirty state
func ForceVersion(db *sql.DB, dbName string, version int) error {
	sourceDriver, err := iofs.New(MigrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	databaseDriver, err := postgres.WithInstance(db, &postgres.Config{
		DatabaseName: dbName,
	})
	if err != nil {
		return fmt.Errorf("failed to create database driver: %w", err)
	}

	m, err := migrate.NewWithInstance(
		"iofs",
		sourceDriver,
		dbName,
		databaseDriver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	if err := m.Force(version); err != nil {
		return fmt.Errorf("failed to force version: %w", err)
	}

	return nil
}

// RollbackMigrations rolls back N migrations
func RollbackMigrations(db *sql.DB, dbName string, steps int) error {
	sourceDriver, err := iofs.New(MigrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	databaseDriver, err := postgres.WithInstance(db, &postgres.Config{
		DatabaseName: dbName,
	})
	if err != nil {
		return fmt.Errorf("failed to create database driver: %w", err)
	}

	m, err := migrate.NewWithInstance(
		"iofs",
		sourceDriver,
		dbName,
		databaseDriver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	if err := m.Steps(-steps); err != nil {
		return fmt.Errorf("failed to rollback migrations: %w", err)
	}

	return nil
}

// GetMigrationVersion returns current migration version and dirty state
func GetMigrationVersion(db *sql.DB, dbName string) (uint, bool, error) {
	sourceDriver, err := iofs.New(MigrationFiles, "migrations")
	if err != nil {
		return 0, false, fmt.Errorf("failed to create migration source: %w", err)
	}

	databaseDriver, err := postgres.WithInstance(db, &postgres.Config{
		DatabaseName: dbName,
	})
	if err != nil {
		return 0, false, fmt.Errorf("failed to create database driver: %w", err)
	}

	m, err := migrate.NewWithInstance(
		"iofs",
		sourceDriver,
		dbName,
		databaseDriver,
	)
	if err != nil {
		return 0, false, fmt.Errorf("failed to create migrator: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil {
		if err == migrate.ErrNilVersion {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("failed to get migration version: %w", err)
	}

	return version, dirty, nil
}