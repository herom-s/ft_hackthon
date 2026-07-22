package database

import (
	"context"
	"fmt"
	"log"
	"sort"
)

type migration struct {
	ID          int
	Description string
	SQL         string
}

var migrations = []migration{
	{
		ID:          1,
		Description: "create users table",
		SQL: `CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			email TEXT NOT NULL DEFAULT '',
			password TEXT NOT NULL DEFAULT '',
			gitea_repo_url TEXT NOT NULL DEFAULT '',
			gitea_token TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
	},
	{
		ID:          2,
		Description: "create jobs table",
		SQL: `CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			commit_sha TEXT NOT NULL DEFAULT '',
			gitea_clone_url TEXT NOT NULL DEFAULT '',
			suite TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'queued',
			message TEXT NOT NULL DEFAULT '',
			result JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
	},
	{
		ID:          3,
		Description: "create tokens table",
		SQL: `CREATE TABLE IF NOT EXISTS tokens (
			token TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id)
		)`,
	},
	{
		ID:          4,
		Description: "add rating column to users",
		SQL:         `ALTER TABLE users ADD COLUMN IF NOT EXISTS rating INT NOT NULL DEFAULT 1200`,
	},
	{
		ID:          5,
		Description: "add claimed_by and claimed_at to jobs for safe concurrent workers",
		SQL: `ALTER TABLE jobs ADD COLUMN IF NOT EXISTS claimed_by TEXT NOT NULL DEFAULT '';
		ALTER TABLE jobs ADD COLUMN IF NOT EXISTS claimed_at TIMESTAMPTZ`,
	},
	{
		ID:          6,
		Description: "add indexes on frequently queried fields",
		SQL: `CREATE INDEX IF NOT EXISTS idx_jobs_user_id ON jobs(user_id);
		CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
		CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at);
		CREATE INDEX IF NOT EXISTS idx_jobs_suite ON jobs(suite);
		CREATE INDEX IF NOT EXISTS idx_tokens_user_id ON tokens(user_id);
		CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
	},
}

const schemaMigrationsTable = "schema_migrations"

func (db *PostgresDB) migrate() error {
	ctx := context.Background()

	if err := db.ensureMigrationsTable(ctx); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	applied, err := db.getAppliedMigrations(ctx)
	if err != nil {
		return fmt.Errorf("get applied migrations: %w", err)
	}

	// Sort migrations by ID
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].ID < migrations[j].ID
	})

	for _, m := range migrations {
		if applied[m.ID] {
			continue
		}

		log.Printf("Applying migration %d: %s", m.ID, m.Description)

		if _, err := db.pool.Exec(ctx, m.SQL); err != nil {
			return fmt.Errorf("migration %d (%s): %w", m.ID, m.Description, err)
		}

		if _, err := db.pool.Exec(ctx,
			`INSERT INTO `+schemaMigrationsTable+` (id, description) VALUES ($1, $2)`,
			m.ID, m.Description,
		); err != nil {
			return fmt.Errorf("record migration %d: %w", m.ID, err)
		}

		log.Printf("Migration %d applied successfully", m.ID)
	}

	return nil
}

func (db *PostgresDB) ensureMigrationsTable(ctx context.Context) error {
	_, err := db.pool.Exec(ctx,
		`CREATE TABLE IF NOT EXISTS `+schemaMigrationsTable+` (
			id INT PRIMARY KEY,
			description TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
	)
	return err
}

func (db *PostgresDB) getAppliedMigrations(ctx context.Context) (map[int]bool, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT id FROM `+schemaMigrationsTable+` ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		applied[id] = true
	}
	return applied, nil
}
