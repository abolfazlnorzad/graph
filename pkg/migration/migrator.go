package migration

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"sort"

	"github.com/abolfazlnorzad/graph/pkg/postgresdb"
	migrate "github.com/rubenv/sql-migrate"

	_ "github.com/lib/pq"
)

var registeredMigrations []embed.FS

// Register is called by modules in their init() to register their embedded migrations.
func Register(fs embed.FS) {
	registeredMigrations = append(registeredMigrations, fs)
}

type Migrator struct {
	dialect  string
	dbConfig postgresdb.Config
	source   migrate.MigrationSource
}

// NewFromEmbed combines all registered embedded filesystems into a single global sorted migration source.
func NewFromEmbed(dbConfig postgresdb.Config) (Migrator, error) {
	var allMigrations []*migrate.Migration

	for _, efs := range registeredMigrations {
		source := migrate.EmbedFileSystemMigrationSource{
			FileSystem: efs,
			Root:       ".",
		}

		migrations, err := source.FindMigrations()
		if err != nil {
			return Migrator{}, fmt.Errorf("failed to find migrations in embed.FS: %w", err)
		}

		allMigrations = append(allMigrations, migrations...)
	}

	sort.Slice(allMigrations, func(i, j int) bool {
		return allMigrations[i].Id < allMigrations[j].Id
	})

	memorySource := &migrate.MemoryMigrationSource{
		Migrations: allMigrations,
	}

	return Migrator{
		dialect:  "postgres",
		dbConfig: dbConfig,
		source:   memorySource,
	}, nil
}

func (m Migrator) Up() error {
	connStr := postgresdb.BuildDSN(m.dbConfig)

	db, err := sql.Open(m.dialect, connStr)
	if err != nil {
		return fmt.Errorf("can't open %s db: %w", m.dialect, err)
	}
	defer func() {
		err = errors.Join(err, db.Close())
	}()

	n, err := migrate.Exec(db, m.dialect, m.source, migrate.Up)
	if err != nil {
		return fmt.Errorf("can't apply migrations up: %w", err)
	}

	fmt.Printf("Applied %d migrations!\n", n)
	return nil
}

func (m Migrator) Down() (err error) {
	connStr := postgresdb.BuildDSN(m.dbConfig)

	db, err := sql.Open(m.dialect, connStr)
	if err != nil {
		return fmt.Errorf("can't open %s db: %w", m.dialect, err)
	}

	defer func() {
		closeErr := db.Close()
		if closeErr != nil {
			if err == nil {
				err = fmt.Errorf("failed to close db: %w", closeErr)
			} else {
				fmt.Printf("warning: failed to close db: %v\n", closeErr)
			}
		}
	}()

	n, execErr := migrate.Exec(db, m.dialect, m.source, migrate.Down)
	if execErr != nil {
		return fmt.Errorf("can't apply migrations down: %w", execErr)
	}

	fmt.Printf("Rolled back %d migrations!\n", n)
	return nil
}
