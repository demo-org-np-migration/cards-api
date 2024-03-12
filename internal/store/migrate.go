package store

import (
	"errors"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations applies every pending migration under ./migrations against
// databaseURL. cards-api owns the `cards` database (las convenciones internas de API), so it
// runs its own migrations at boot instead of relying on someone else having
// done it first.
//
// migrationsPath is a file:// source URL. In the container it's
// "file:///app/migrations" (see the Dockerfile); in local dev and tests it
// defaults to the repo-relative "file://migrations".
func RunMigrations(databaseURL, migrationsPath string) error {
	m, err := migrate.New(migrationsPath, databaseURL)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}
