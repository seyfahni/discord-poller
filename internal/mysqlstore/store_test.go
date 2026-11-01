package mysqlstore

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/seyfahni/discord-poller/internal/app"
)

func newMockStore(t *testing.T) (*Store, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	t.Cleanup(func() {
		mock.ExpectClose()
		if err := db.Close(); err != nil {
			t.Errorf("close mock database: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet database expectations: %v", err)
		}
	})
	return &Store{db: db}, mock
}

func TestServerSettings_canBeSavedAndLoaded(t *testing.T) {
	store, mock := newMockStore(t)
	want := app.ServerSettings{
		GuildID:             "123456789012345678",
		GroupCreationRoleID: "234567890123456789",
	}

	mock.ExpectExec("INSERT INTO server_settings").
		WithArgs(want.GuildID, want.GroupCreationRoleID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveServerSettings(context.Background(), want); err != nil {
		t.Fatalf("SaveServerSettings() error = %v", err)
	}

	mock.ExpectQuery("SELECT guild_id, group_creation_role_id FROM server_settings").
		WithArgs(want.GuildID).
		WillReturnRows(sqlmock.NewRows([]string{"guild_id", "group_creation_role_id"}).
			AddRow(want.GuildID, want.GroupCreationRoleID))
	got, found, err := store.GetServerSettings(context.Background(), want.GuildID)
	if err != nil {
		t.Fatalf("GetServerSettings() error = %v", err)
	}
	if !found || got != want {
		t.Fatalf("GetServerSettings() = (%#v, %v), want (%#v, true)", got, found, want)
	}
}

func TestScheduledJobs_areSavedAndLoadedInRunTimeOrder(t *testing.T) {
	store, mock := newMockStore(t)
	earlier := app.ScheduledJob{
		ID:      "job-earlier",
		GuildID: "123456789012345678",
		Kind:    "open_poll",
		RunAt:   time.Date(2026, 6, 1, 18, 0, 0, 0, time.UTC),
		Payload: json.RawMessage(`{"group_id":"group-1"}`),
	}
	later := app.ScheduledJob{
		ID:      "job-later",
		GuildID: "123456789012345678",
		Kind:    "open_poll",
		RunAt:   time.Date(2026, 6, 2, 18, 0, 0, 0, time.UTC),
		Payload: json.RawMessage(`{"group_id":"group-2"}`),
	}
	mock.ExpectExec("INSERT INTO scheduled_jobs").
		WithArgs(later.ID, later.GuildID, later.Kind, later.RunAt, []byte(later.Payload)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.SaveScheduledJob(context.Background(), later); err != nil {
		t.Fatalf("SaveScheduledJob() error = %v", err)
	}
	mock.ExpectQuery("SELECT id, guild_id, kind, run_at, payload FROM scheduled_jobs").
		WillReturnRows(sqlmock.NewRows([]string{"id", "guild_id", "kind", "run_at", "payload"}).
			AddRow(earlier.ID, earlier.GuildID, earlier.Kind, earlier.RunAt, []byte(earlier.Payload)).
			AddRow(later.ID, later.GuildID, later.Kind, later.RunAt, []byte(later.Payload)))

	got, err := store.ListScheduledJobs(context.Background())
	if err != nil {
		t.Fatalf("ListScheduledJobs() error = %v", err)
	}
	if len(got) != 2 || got[0].ID != earlier.ID || got[1].ID != later.ID {
		t.Fatalf("ListScheduledJobs() = %#v, want jobs ordered by run time", got)
	}
	if string(got[0].Payload) != string(earlier.Payload) {
		t.Fatalf("first job payload = %s, want %s", got[0].Payload, earlier.Payload)
	}
}

func TestMigrate_createsServerSettingsAndScheduledJobTables(t *testing.T) {
	store, mock := newMockStore(t)

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS server_settings").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS scheduled_jobs").
		WillReturnResult(sqlmock.NewResult(0, 0))
	if err := store.migrate(context.Background()); err != nil {
		t.Fatalf("migrate() error = %v", err)
	}
}
