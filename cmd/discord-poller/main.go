package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/seyfahni/discord-poller/internal/app"
	"github.com/seyfahni/discord-poller/internal/config"
	"github.com/seyfahni/discord-poller/internal/discord"
	"github.com/seyfahni/discord-poller/internal/mysqlstore"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	gateway, err := discord.New(cfg.DiscordToken)
	if err != nil {
		return err
	}
	store, err := mysqlstore.Open(context.Background(), cfg.MySQLDSN)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runErr := app.New(store, gateway).Run(ctx)
	if closeErr := store.Close(); closeErr != nil {
		return errors.Join(runErr, fmt.Errorf("close storage: %w", closeErr))
	}
	return runErr
}
