// Package app coordinates the bot's application lifecycle.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Store provides the application data needed to operate the service.
type Store interface {
	Ping(context.Context) error
	GetServerSettings(context.Context, string) (ServerSettings, bool, error)
	SaveServerSettings(context.Context, ServerSettings) error
	ListScheduledJobs(context.Context) ([]ScheduledJob, error)
	SaveScheduledJob(context.Context, ScheduledJob) error
	DeleteScheduledJob(context.Context, string) error
}

// ServerSettings contains server-scoped bot configuration.
type ServerSettings struct {
	GuildID             string
	GroupCreationRoleID string
}

// ScheduledJob contains persistent work the service must resume after restart.
type ScheduledJob struct {
	ID      string
	GuildID string
	Kind    string
	RunAt   time.Time
	Payload json.RawMessage
}

// Validate checks that a scheduled job can be safely persisted.
func (job ScheduledJob) Validate() error {
	if job.ID == "" || job.GuildID == "" || job.Kind == "" || job.RunAt.IsZero() {
		return errors.New("scheduled job requires an ID, server, kind, and run time")
	}
	if !json.Valid(job.Payload) {
		return errors.New("scheduled job payload must be valid JSON")
	}
	return nil
}

// Gateway manages the connection to Discord.
type Gateway interface {
	Open() error
	Close() error
}

// Application runs the bot against its storage and Discord gateway.
type Application struct {
	store   Store
	gateway Gateway
}

// New creates an application with its storage and Discord gateway.
func New(store Store, gateway Gateway) *Application {
	return &Application{store: store, gateway: gateway}
}

// Run connects to Discord and shuts down when the context is canceled.
func (a *Application) Run(ctx context.Context) error {
	if err := a.store.Ping(ctx); err != nil {
		return fmt.Errorf("check storage: %w", err)
	}
	if err := a.gateway.Open(); err != nil {
		return fmt.Errorf("open Discord gateway: %w", err)
	}

	<-ctx.Done()

	if err := a.gateway.Close(); err != nil {
		return fmt.Errorf("close Discord gateway: %w", err)
	}
	return nil
}
