// Package mysqlstore provides MySQL-backed application storage.
package mysqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/seyfahni/discord-poller/internal/app"
)

var _ app.Store = (*Store)(nil)

// Store persists application data in MySQL.
type Store struct {
	db *sql.DB
}

// Open connects to MySQL, verifies the connection, and applies the schema.
func Open(ctx context.Context, dsn string) (*Store, error) {
	driverConfig, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse MySQL DSN: %w", err)
	}
	driverConfig.ParseTime = true
	driverConfig.Loc = time.UTC

	db, err := sql.Open("mysql", driverConfig.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("open MySQL connection: %w", err)
	}
	db.SetConnMaxLifetime(3 * time.Minute)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	store := &Store{db: db}
	if err := store.Ping(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to MySQL: %w", err)
	}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate MySQL schema: %w", err)
	}
	return store, nil
}

// Close closes the underlying database connection pool.
func (s *Store) Close() error {
	return s.db.Close()
}

// Ping verifies that MySQL is reachable.
func (s *Store) Ping(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping MySQL: %w", err)
	}
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS server_settings (
			guild_id VARCHAR(20) NOT NULL PRIMARY KEY,
			group_creation_role_id VARCHAR(20) NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS scheduled_jobs (
			id VARCHAR(64) NOT NULL PRIMARY KEY,
			guild_id VARCHAR(20) NOT NULL,
			kind VARCHAR(64) NOT NULL,
			run_at DATETIME(6) NOT NULL,
			payload JSON NOT NULL,
			INDEX scheduled_jobs_run_at_idx (run_at),
			INDEX scheduled_jobs_guild_id_idx (guild_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("execute schema statement: %w", err)
		}
	}
	return nil
}

// GetServerSettings returns the settings for a server when they exist.
func (s *Store) GetServerSettings(ctx context.Context, guildID string) (app.ServerSettings, bool, error) {
	var settings app.ServerSettings
	err := s.db.QueryRowContext(ctx,
		"SELECT guild_id, group_creation_role_id FROM server_settings WHERE guild_id = ?",
		guildID,
	).Scan(&settings.GuildID, &settings.GroupCreationRoleID)
	if errors.Is(err, sql.ErrNoRows) {
		return app.ServerSettings{}, false, nil
	}
	if err != nil {
		return app.ServerSettings{}, false, fmt.Errorf("get server settings: %w", err)
	}
	return settings, true, nil
}

// SaveServerSettings stores settings for a server.
func (s *Store) SaveServerSettings(ctx context.Context, settings app.ServerSettings) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO server_settings (guild_id, group_creation_role_id)
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE group_creation_role_id = VALUES(group_creation_role_id)`,
		settings.GuildID, settings.GroupCreationRoleID,
	)
	if err != nil {
		return fmt.Errorf("save server settings: %w", err)
	}
	return nil
}

// ListScheduledJobs returns stored work ordered by its scheduled time.
func (s *Store) ListScheduledJobs(ctx context.Context) ([]app.ScheduledJob, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, guild_id, kind, run_at, payload FROM scheduled_jobs ORDER BY run_at, id",
	)
	if err != nil {
		return nil, fmt.Errorf("list scheduled jobs: %w", err)
	}
	defer rows.Close()

	var jobs []app.ScheduledJob
	for rows.Next() {
		var job app.ScheduledJob
		var payload []byte
		if err := rows.Scan(&job.ID, &job.GuildID, &job.Kind, &job.RunAt, &payload); err != nil {
			return nil, fmt.Errorf("scan scheduled job: %w", err)
		}
		job.RunAt = job.RunAt.UTC()
		job.Payload = append(job.Payload[:0], payload...)
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scheduled jobs: %w", err)
	}
	return jobs, nil
}

// SaveScheduledJob stores or replaces scheduled work by its ID.
func (s *Store) SaveScheduledJob(ctx context.Context, job app.ScheduledJob) error {
	if err := job.Validate(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO scheduled_jobs (id, guild_id, kind, run_at, payload)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			guild_id = VALUES(guild_id),
			kind = VALUES(kind),
			run_at = VALUES(run_at),
			payload = VALUES(payload)`,
		job.ID, job.GuildID, job.Kind, job.RunAt.UTC(), []byte(job.Payload),
	)
	if err != nil {
		return fmt.Errorf("save scheduled job: %w", err)
	}
	return nil
}

// DeleteScheduledJob removes scheduled work by its ID.
func (s *Store) DeleteScheduledJob(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM scheduled_jobs WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete scheduled job: %w", err)
	}
	return nil
}
