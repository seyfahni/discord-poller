package memory_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/seyfahni/discord-poller/internal/app"
	"github.com/seyfahni/discord-poller/internal/memory"
)

func TestServerSettings_canBeSavedAndLoaded(t *testing.T) {
	store := memory.New()
	want := app.ServerSettings{
		GuildID:             "123456789012345678",
		GroupCreationRoleID: "234567890123456789",
	}

	if err := store.SaveServerSettings(context.Background(), want); err != nil {
		t.Fatalf("SaveServerSettings() error = %v", err)
	}
	got, found, err := store.GetServerSettings(context.Background(), want.GuildID)
	if err != nil {
		t.Fatalf("GetServerSettings() error = %v", err)
	}
	if !found || got != want {
		t.Fatalf("GetServerSettings() = (%#v, %v), want (%#v, true)", got, found, want)
	}
}

func TestScheduledJobs_areLoadedInRunTimeOrder(t *testing.T) {
	store := memory.New()
	later := app.ScheduledJob{
		ID:      "job-later",
		GuildID: "123456789012345678",
		Kind:    "open_poll",
		RunAt:   time.Date(2026, 6, 2, 18, 0, 0, 0, time.UTC),
		Payload: json.RawMessage(`{"group_id":"group-2"}`),
	}
	earlier := app.ScheduledJob{
		ID:      "job-earlier",
		GuildID: "123456789012345678",
		Kind:    "open_poll",
		RunAt:   time.Date(2026, 6, 1, 18, 0, 0, 0, time.UTC),
		Payload: json.RawMessage(`{"group_id":"group-1"}`),
	}
	for _, job := range []app.ScheduledJob{later, earlier} {
		if err := store.SaveScheduledJob(context.Background(), job); err != nil {
			t.Fatalf("SaveScheduledJob(%q) error = %v", job.ID, err)
		}
	}

	got, err := store.ListScheduledJobs(context.Background())
	if err != nil {
		t.Fatalf("ListScheduledJobs() error = %v", err)
	}
	if want := []app.ScheduledJob{earlier, later}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ListScheduledJobs() = %#v, want %#v", got, want)
	}
}

func TestScheduledJob_rejectsInvalidPayload(t *testing.T) {
	job := app.ScheduledJob{
		ID:      "job-invalid",
		GuildID: "123456789012345678",
		Kind:    "open_poll",
		RunAt:   time.Date(2026, 6, 1, 18, 0, 0, 0, time.UTC),
		Payload: json.RawMessage(`not-json`),
	}

	if err := memory.New().SaveScheduledJob(context.Background(), job); err == nil {
		t.Fatal("SaveScheduledJob() error = nil, want invalid payload error")
	}
}
