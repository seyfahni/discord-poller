package mysqlstore_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/seyfahni/discord-poller/internal/app"
	"github.com/seyfahni/discord-poller/internal/mysqlstore"
)

func TestPersistence_survivesReopeningStore(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to run the MySQL persistence integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	guildID := fmt.Sprintf("%d", time.Now().UnixNano())
	jobID := "integration-" + guildID
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := db.ExecContext(cleanupCtx, "DELETE FROM scheduled_jobs WHERE id = ?", jobID); err != nil {
			t.Errorf("delete integration job: %v", err)
		}
		if _, err := db.ExecContext(cleanupCtx, "DELETE FROM server_settings WHERE guild_id = ?", guildID); err != nil {
			t.Errorf("delete integration settings: %v", err)
		}
		if err := db.Close(); err != nil {
			t.Errorf("close integration database: %v", err)
		}
	})

	store, err := mysqlstore.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("mysqlstore.Open() error = %v", err)
	}
	wantSettings := app.ServerSettings{
		GuildID:             guildID,
		GroupCreationRoleID: guildID,
	}
	if err := store.SaveServerSettings(ctx, wantSettings); err != nil {
		t.Fatalf("SaveServerSettings() error = %v", err)
	}
	wantJob := app.ScheduledJob{
		ID:      jobID,
		GuildID: guildID,
		Kind:    "integration",
		RunAt:   time.Date(2026, 6, 1, 18, 0, 0, 0, time.UTC),
		Payload: json.RawMessage(`{"integration":true}`),
	}
	if err := store.SaveScheduledJob(ctx, wantJob); err != nil {
		t.Fatalf("SaveScheduledJob() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	store, err = mysqlstore.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("reopen MySQL store: %v", err)
	}
	defer store.Close()
	gotSettings, found, err := store.GetServerSettings(ctx, guildID)
	if err != nil {
		t.Fatalf("GetServerSettings() error = %v", err)
	}
	if !found || gotSettings != wantSettings {
		t.Fatalf("GetServerSettings() = (%#v, %v), want (%#v, true)", gotSettings, found, wantSettings)
	}
	jobs, err := store.ListScheduledJobs(ctx)
	if err != nil {
		t.Fatalf("ListScheduledJobs() error = %v", err)
	}
	for _, job := range jobs {
		if job.ID == jobID {
			if job.GuildID != wantJob.GuildID || job.Kind != wantJob.Kind ||
				!job.RunAt.Equal(wantJob.RunAt) || string(job.Payload) != string(wantJob.Payload) {
				t.Fatalf("persisted job = %#v, want %#v", job, wantJob)
			}
			return
		}
	}
	t.Fatalf("ListScheduledJobs() did not return persisted job %q", jobID)
}
