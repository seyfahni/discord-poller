// Package memory provides an in-memory implementation of application storage.
package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/seyfahni/discord-poller/internal/app"
)

// Store is an in-memory application store.
type Store struct {
	mu       sync.RWMutex
	settings map[string]app.ServerSettings
	jobs     map[string]app.ScheduledJob
}

// New creates an in-memory store.
func New() *Store {
	return &Store{
		settings: make(map[string]app.ServerSettings),
		jobs:     make(map[string]app.ScheduledJob),
	}
}

// Ping reports whether the caller's context is still active.
func (*Store) Ping(ctx context.Context) error {
	return ctx.Err()
}

// GetServerSettings returns the settings for a server when they exist.
func (s *Store) GetServerSettings(ctx context.Context, guildID string) (app.ServerSettings, bool, error) {
	if err := ctx.Err(); err != nil {
		return app.ServerSettings{}, false, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	settings, found := s.settings[guildID]
	return settings, found, nil
}

// SaveServerSettings stores settings for a server.
func (s *Store) SaveServerSettings(ctx context.Context, settings app.ServerSettings) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.settings[settings.GuildID] = settings
	return nil
}

// ListScheduledJobs returns stored work ordered by its scheduled time.
func (s *Store) ListScheduledJobs(ctx context.Context) ([]app.ScheduledJob, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]app.ScheduledJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		job.Payload = append([]byte(nil), job.Payload...)
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].RunAt.Equal(jobs[j].RunAt) {
			return jobs[i].ID < jobs[j].ID
		}
		return jobs[i].RunAt.Before(jobs[j].RunAt)
	})
	return jobs, nil
}

// SaveScheduledJob stores or replaces scheduled work by its ID.
func (s *Store) SaveScheduledJob(ctx context.Context, job app.ScheduledJob) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := job.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	job.Payload = append([]byte(nil), job.Payload...)
	s.jobs[job.ID] = job
	return nil
}

// DeleteScheduledJob removes scheduled work by its ID.
func (s *Store) DeleteScheduledJob(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.jobs, id)
	return nil
}
