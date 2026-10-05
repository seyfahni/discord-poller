package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/seyfahni/discord-poller/internal/config"
	"github.com/seyfahni/discord-poller/internal/discord"
	"github.com/seyfahni/discord-poller/internal/logger"
	"github.com/seyfahni/discord-poller/internal/schedule"
	polltemplate "github.com/seyfahni/discord-poller/internal/template"
	"go.uber.org/zap"
)

const (
	exitSuccess       = 0
	exitInternal      = 1
	exitConfiguration = 2
	exitTransport     = 3
	exitAPI           = 4
)

type discordClient interface {
	ValidateWebhook(context.Context, string) (*discord.Webhook, error)
	CreatePollWithPayload(context.Context, string, func() (discord.Payload, error)) (*discord.CreatedMessage, error)
}

type cliDependencies struct {
	now           func() time.Time
	loggerFactory func(string) (*zap.Logger, error)
	clientFactory func(*http.Client, int, time.Duration) (discordClient, error)
	stdout        io.Writer
	stderr        io.Writer
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], productionDependencies())
	stop()
	os.Exit(code)
}

func productionDependencies() cliDependencies {
	return cliDependencies{
		now: time.Now,
		loggerFactory: func(level string) (*zap.Logger, error) {
			return logger.New(level)
		},
		clientFactory: func(client *http.Client, maxRetries int, maxWait time.Duration) (discordClient, error) {
			return discord.NewClient(client, maxRetries, maxWait)
		},
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
}

func run(ctx context.Context, args []string, dependencies cliDependencies) int {
	if dependencies.stdout == nil {
		dependencies.stdout = os.Stdout
	}
	flags := flag.NewFlagSet("discord-poller", flag.ContinueOnError)
	output := dependencies.stderr
	if output == nil {
		output = io.Discard
	}
	flags.SetOutput(output)
	flags.Usage = func() {
		_, _ = fmt.Fprintln(flags.Output(), "Usage: discord-poller --config USER_CONFIG --system-config SYSTEM_CONFIG [--dry-run]")
		flags.PrintDefaults()
	}
	userConfigPath := flags.String("config", "", "path to the user configuration YAML")
	systemConfigPath := flags.String("system-config", "", "path to the system configuration YAML")
	dryRun := flags.Bool("dry-run", false, "validate configuration and webhook without creating a poll")
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			_, _ = fmt.Fprintln(dependencies.stdout, "Usage: discord-poller --config USER_CONFIG --system-config SYSTEM_CONFIG [--dry-run]")
			_, _ = fmt.Fprintln(dependencies.stdout, "  --config         path to the user configuration YAML")
			_, _ = fmt.Fprintln(dependencies.stdout, "  --system-config  path to the system configuration YAML")
			_, _ = fmt.Fprintln(dependencies.stdout, "  --dry-run        validate configuration and webhook without creating a poll")
			return exitSuccess
		}
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitSuccess
		}
		return exitConfiguration
	}
	if flags.NArg() > 0 || *userConfigPath == "" || *systemConfigPath == "" {
		flags.Usage()
		return exitConfiguration
	}
	if dependencies.now == nil {
		dependencies.now = time.Now
	}
	if dependencies.stdout == nil {
		dependencies.stdout = io.Discard
	}
	if dependencies.stderr == nil {
		dependencies.stderr = io.Discard
	}

	systemConfig, err := config.LoadSystemConfig(*systemConfigPath)
	if err != nil {
		_, _ = fmt.Fprintf(dependencies.stderr, "system configuration error: %v\n", err)
		return exitConfiguration
	}
	userConfig, err := config.LoadUserConfig(*userConfigPath)
	if err != nil {
		_, _ = fmt.Fprintf(dependencies.stderr, "user configuration error: %v\n", err)
		return exitConfiguration
	}

	loggerFactory := dependencies.loggerFactory
	if loggerFactory == nil {
		loggerFactory = logger.New
	}
	log, err := loggerFactory(systemConfig.Logging.Level)
	if err != nil {
		_, _ = fmt.Fprintln(dependencies.stderr, "failed to initialize logger")
		return exitInternal
	}
	defer log.Sync()

	return runConfigured(ctx, userConfig, systemConfig, *userConfigPath, *dryRun, dependencies, log)
}

func runConfigured(ctx context.Context, userConfig *config.UserConfig, systemConfig *config.SystemConfig, configPath string, dryRun bool, dependencies cliDependencies, log *zap.Logger) int {
	webhookURL := userConfig.WebhookURL
	if err := discord.ValidateWebhookURL(webhookURL); err != nil {
		log.Error("webhook URL is invalid")
		return exitConfiguration
	}

	location, err := time.LoadLocation(userConfig.Target.Timezone)
	if err != nil {
		log.Error("target timezone is invalid", zap.Error(err))
		return exitConfiguration
	}
	weekday, err := parseWeekday(userConfig.Target.Weekday)
	if err != nil {
		log.Error("target weekday is invalid", zap.Error(err))
		return exitConfiguration
	}
	targetTime, err := time.Parse("15:04", userConfig.Target.Time)
	if err != nil {
		log.Error("target time is invalid", zap.Error(err))
		return exitConfiguration
	}

	deadline, err := schedule.SelectDeadline(
		dependencies.now(),
		location,
		weekday,
		targetTime.Hour(),
		targetTime.Minute(),
		userConfig.Target.SameDayThreshold,
	)
	if err != nil {
		log.Error("failed to select target deadline", zap.Error(err))
		return exitConfiguration
	}

	question, err := polltemplate.Render(userConfig.Poll.Question, deadline.Format(userConfig.Poll.DateFormat))
	if err != nil {
		log.Error("failed to render poll question", zap.Error(err))
		return exitConfiguration
	}
	answers := make([]discord.Answer, 0, len(userConfig.Poll.Answers))
	for index, answer := range userConfig.Poll.Answers {
		text, err := polltemplate.Render(answer.Text, deadline.Format(userConfig.Poll.DateFormat))
		if err != nil {
			log.Error("failed to render poll answer", zap.Int("answer_index", index), zap.Error(err))
			return exitConfiguration
		}
		answers = append(answers, discord.Answer{Text: text, Emoji: answer.Emoji})
	}

	buildPayload := func() (discord.Payload, error) {
		durationHours, err := schedule.DurationHours(deadline, dependencies.now())
		if err != nil {
			return discord.Payload{}, err
		}
		payload, err := discord.BuildPayload(question, answers, durationHours, userConfig.Poll.AllowMultiselect)
		if err != nil {
			return discord.Payload{}, err
		}
		log.Info("poll payload validated", zap.Int("duration_hours", durationHours))
		return payload, nil
	}

	payload, err := buildPayload()
	if err != nil {
		log.Error("poll payload validation failed", zap.Error(err))
		return exitConfiguration
	}

	httpClient := &http.Client{Timeout: systemConfig.HTTP.RequestTimeout}
	clientFactory := dependencies.clientFactory
	if clientFactory == nil {
		clientFactory = func(client *http.Client, maxRetries int, maxWait time.Duration) (discordClient, error) {
			return discord.NewClient(client, maxRetries, maxWait)
		}
	}
	client, err := clientFactory(httpClient, systemConfig.HTTP.MaxRetries, systemConfig.HTTP.MaxRateLimitWait)
	if err != nil {
		log.Error("failed to initialize Discord client")
		return exitInternal
	}

	log.Info("configuration loaded", zap.String("config_file", configPath))
	log.Info("target deadline selected",
		zap.Time("deadline", deadline),
		zap.String("target_deadline", deadline.Format(time.RFC3339)),
	)
	if _, err := client.ValidateWebhook(ctx, webhookURL); err != nil {
		return logDiscordError(log, "webhook validation", err)
	}
	log.Info("webhook credential validated")
	if dryRun {
		log.Info("dry run complete; poll was not sent",
			zap.Bool("dry_run", true),
			zap.Time("deadline", deadline),
			zap.Int("duration_hours", payload.Poll.Duration),
			zap.Any("payload", payload),
		)
		return exitSuccess
	}

	message, err := client.CreatePollWithPayload(ctx, webhookURL, buildPayload)
	if err != nil {
		return logDiscordError(log, "poll creation", err)
	}
	if message == nil {
		log.Error("Discord returned no created message")
		return exitTransport
	}

	fields := []zap.Field{zap.String("message_id", message.ID)}
	if message.Poll == nil || message.Poll.Expiry == nil {
		fields = append(fields, zap.Bool("poll_expiry_unavailable", true))
		log.Info("poll created", fields...)
		return exitSuccess
	}

	expiry := *message.Poll.Expiry
	fields = append(fields, zap.Time("poll_expiry", expiry))
	if !expiry.After(deadline.Add(-time.Hour)) || expiry.After(deadline) {
		deviation := expiry.Sub(deadline).Seconds()
		log.Warn("poll expiry is outside the intended window",
			append(fields,
				zap.Bool("expiry_outside_window", true),
				zap.Float64("deviation_seconds", deviation),
			)...,
		)
		return exitSuccess
	}
	log.Info("poll created", fields...)
	return exitSuccess
}

func logDiscordError(log *zap.Logger, operation string, err error) int {
	switch {
	case errors.Is(err, discord.ErrConfiguration):
		log.Error(operation + " configuration failed")
		return exitConfiguration
	case errors.Is(err, discord.ErrAPI):
		log.Error(operation+" was rejected by Discord", zap.Error(err))
		return exitAPI
	case errors.Is(err, discord.ErrTransport):
		log.Error(operation+" failed or has an ambiguous outcome", zap.Error(err))
		return exitTransport
	default:
		log.Error(operation + " failed")
		return exitInternal
	}
}

func parseWeekday(name string) (time.Weekday, error) {
	for weekday := time.Sunday; weekday <= time.Saturday; weekday++ {
		if weekday.String() == name {
			return weekday, nil
		}
	}
	return 0, fmt.Errorf("unknown weekday")
}
