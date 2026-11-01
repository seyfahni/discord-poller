package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/seyfahni/discord-poller/internal/app"
	"github.com/seyfahni/discord-poller/internal/memory"
)

func TestRun_startsAndStopsDiscordWhenContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	gateway := &fakeGateway{onOpen: cancel}
	application := app.New(memory.New(), gateway)

	if err := application.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !gateway.opened || !gateway.closed {
		t.Fatalf("gateway lifecycle = opened %v, closed %v; want both true", gateway.opened, gateway.closed)
	}
}

func TestRun_doesNotConnectWhenStorageIsUnavailable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	gateway := &fakeGateway{}
	application := app.New(failingStore{
		Store: memory.New(),
		err:   errors.New("database unavailable"),
	}, gateway)

	if err := application.Run(ctx); err == nil {
		t.Fatal("Run() error = nil, want storage error")
	}
	if gateway.opened {
		t.Fatal("gateway opened while storage was unavailable")
	}
}

type fakeGateway struct {
	onOpen func()
	opened bool
	closed bool
}

func (g *fakeGateway) Open() error {
	g.opened = true
	if g.onOpen != nil {
		g.onOpen()
	}
	return nil
}

func (g *fakeGateway) Close() error {
	g.closed = true
	return nil
}

type failingStore struct {
	app.Store
	err error
}

func (s failingStore) Ping(context.Context) error {
	return s.err
}
