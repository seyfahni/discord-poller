package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seyfahni/discord-poller/internal/discord"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestRunDryRunValidatesWebhookWithoutCreatingPoll(t *testing.T) {
	files := writeConfigs(t, validUserYAML, validSystemYAML)
	var logs bytes.Buffer
	client := &fakeDiscordClient{webhook: &discord.Webhook{ID: "123456789"}}
	dependencies := testDependencies(t, files, &logs, client)

	code := run(context.Background(), []string{
		"--config", files.user,
		"--system-config", files.system,
		"--dry-run",
	}, dependencies)

	if code != exitSuccess {
		t.Fatalf("run() = %d, want %d; logs: %s", code, exitSuccess, logs.String())
	}
	if client.validateCalls != 1 || client.createCalls != 0 {
		t.Errorf("webhook calls = GET %d, POST %d; want GET 1, POST 0", client.validateCalls, client.createCalls)
	}
	if client.webhookURL != "https://discord.com/api/webhooks/123456789/secret-token" {
		t.Errorf("webhook URL = %q, want configured URL", client.webhookURL)
	}
	for _, field := range []string{"target_deadline", "duration_hours", "dry_run", "payload", "Who's in on 08.10.2026?"} {
		if !strings.Contains(logs.String(), field) {
			t.Errorf("logs missing %q: %s", field, logs.String())
		}
	}
	if strings.Contains(logs.String(), "secret-token") || strings.Contains(logs.String(), "https://discord.com") {
		t.Errorf("logs contain webhook credentials: %s", logs.String())
	}
}

func TestRunCreatesPollAndReportsExpiry(t *testing.T) {
	files := writeConfigs(t, validUserYAML, validSystemYAML)
	var logs bytes.Buffer
	expiry := time.Date(2026, time.October, 8, 15, 30, 0, 0, time.UTC)
	client := &fakeDiscordClient{
		webhook: &discord.Webhook{ID: "123456789"},
		message: &discord.CreatedMessage{
			ID:   "987654321",
			Poll: &discord.CreatedPoll{Expiry: &expiry},
		},
	}
	dependencies := testDependencies(t, files, &logs, client)

	code := run(context.Background(), []string{
		"--config", files.user,
		"--system-config", files.system,
	}, dependencies)

	if code != exitSuccess {
		t.Fatalf("run() = %d, want %d; logs: %s", code, exitSuccess, logs.String())
	}
	if client.validateCalls != 1 || client.createCalls != 1 {
		t.Errorf("webhook calls = GET %d, POST %d; want GET 1, POST 1", client.validateCalls, client.createCalls)
	}
	if client.payload.Poll.Question.Text != "Who's in on 08.10.2026?" {
		t.Errorf("question = %q, want rendered date", client.payload.Poll.Question.Text)
	}
	if got := client.payload.Poll.Answers[0].PollMedia.Text; got != "I'm in on 08.10.2026" {
		t.Errorf("answer = %q, want rendered configured answer", got)
	}
	for _, field := range []string{"message_id", "poll_expiry"} {
		if !strings.Contains(logs.String(), field) {
			t.Errorf("logs missing %q: %s", field, logs.String())
		}
	}
}

func TestRunReportsExpiryDeviationWithoutReposting(t *testing.T) {
	files := writeConfigs(t, validUserYAML, validSystemYAML)
	var logs bytes.Buffer
	expiry := time.Date(2026, time.October, 8, 17, 0, 0, 0, time.UTC)
	client := &fakeDiscordClient{
		webhook: &discord.Webhook{ID: "123456789"},
		message: &discord.CreatedMessage{
			ID:   "987654321",
			Poll: &discord.CreatedPoll{Expiry: &expiry},
		},
	}
	dependencies := testDependencies(t, files, &logs, client)

	code := run(context.Background(), []string{
		"--config", files.user,
		"--system-config", files.system,
	}, dependencies)
	if code != exitSuccess {
		t.Fatalf("run() = %d, want %d; logs: %s", code, exitSuccess, logs.String())
	}
	if client.createCalls != 1 {
		t.Errorf("poll creation calls = %d, want 1", client.createCalls)
	}
	if !strings.Contains(logs.String(), "expiry_outside_window") {
		t.Errorf("logs missing expiry warning: %s", logs.String())
	}
}

func TestRunMissingExpiryIsReported(t *testing.T) {
	files := writeConfigs(t, validUserYAML, validSystemYAML)
	var logs bytes.Buffer
	client := &fakeDiscordClient{
		webhook: &discord.Webhook{ID: "123456789"},
		message: &discord.CreatedMessage{ID: "987654321"},
	}
	dependencies := testDependencies(t, files, &logs, client)

	code := run(context.Background(), []string{
		"--config", files.user,
		"--system-config", files.system,
	}, dependencies)
	if code != exitSuccess {
		t.Fatalf("run() = %d, want %d; logs: %s", code, exitSuccess, logs.String())
	}
	if !strings.Contains(logs.String(), "poll_expiry_unavailable") {
		t.Errorf("logs missing missing-expiry notice: %s", logs.String())
	}
}

func TestRunMapsFailuresToExitCodesWithoutSecrets(t *testing.T) {
	tests := []struct {
		name       string
		client     *fakeDiscordClient
		wantCode   int
		wantLogHas string
	}{
		{
			name:       "webhook configuration error",
			client:     &fakeDiscordClient{validateErr: discord.ErrConfiguration},
			wantCode:   exitConfiguration,
			wantLogHas: "webhook validation",
		},
		{
			name:       "Discord API rejection",
			client:     &fakeDiscordClient{createErr: discord.ErrAPI},
			wantCode:   exitAPI,
			wantLogHas: "poll creation",
		},
		{
			name:       "transport failure",
			client:     &fakeDiscordClient{createErr: discord.ErrTransport},
			wantCode:   exitTransport,
			wantLogHas: "poll creation",
		},
		{
			name:       "request cancellation",
			client:     &fakeDiscordClient{validateErr: errors.Join(discord.ErrTransport, context.Canceled)},
			wantCode:   exitTransport,
			wantLogHas: "webhook validation",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := writeConfigs(t, validUserYAML, validSystemYAML)
			var logs bytes.Buffer
			dependencies := testDependencies(t, files, &logs, test.client)
			code := run(context.Background(), []string{
				"--config", files.user,
				"--system-config", files.system,
			}, dependencies)

			if code != test.wantCode {
				t.Errorf("run() = %d, want %d; logs: %s", code, test.wantCode, logs.String())
			}
			if !strings.Contains(logs.String(), test.wantLogHas) {
				t.Errorf("logs missing %q: %s", test.wantLogHas, logs.String())
			}
			if strings.Contains(logs.String(), "secret-token") {
				t.Errorf("logs contain webhook token: %s", logs.String())
			}
		})
	}
}

func TestRunConfigurationAndUsageErrors(t *testing.T) {
	files := writeConfigs(t, validUserYAML, validSystemYAML)
	tests := []struct {
		name     string
		args     []string
		userYAML string
	}{
		{name: "missing user config", args: []string{"--system-config", files.system}},
		{name: "missing system config", args: []string{"--config", files.user}},
		{name: "missing webhook URL", args: []string{"--config", files.user, "--system-config", files.system}, userYAML: strings.Replace(validUserYAML, "webhook_url: https://discord.com/api/webhooks/123456789/secret-token", "webhook_url: \"\"", 1)},
		{name: "blank webhook URL", args: []string{"--config", files.user, "--system-config", files.system}, userYAML: strings.Replace(validUserYAML, "webhook_url: https://discord.com/api/webhooks/123456789/secret-token", "webhook_url: '  '", 1)},
		{name: "invalid webhook URL", args: []string{"--config", files.user, "--system-config", files.system}, userYAML: strings.Replace(validUserYAML, "https://discord.com/api/webhooks/123456789/secret-token", "https://invalid.example/secret-token", 1)},
		{name: "invalid user config", args: []string{"--config", filepath.Join(t.TempDir(), "missing.yaml"), "--system-config", files.system}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			userYAML := test.userYAML
			if userYAML == "" {
				userYAML = validUserYAML
			}
			configuredFiles := writeConfigs(t, userYAML, validSystemYAML)
			args := append([]string(nil), test.args...)
			for index, arg := range args {
				if arg == files.user {
					args[index] = configuredFiles.user
				}
				if arg == files.system {
					args[index] = configuredFiles.system
				}
			}
			var logs bytes.Buffer
			dependencies := testDependencies(t, configuredFiles, &logs, &fakeDiscordClient{})
			code := run(context.Background(), args, dependencies)
			if code != exitConfiguration {
				t.Errorf("run() = %d, want %d; logs: %s", code, exitConfiguration, logs.String())
			}
			if strings.Contains(logs.String(), "secret-token") {
				t.Errorf("logs contain webhook token: %s", logs.String())
			}
		})
	}
}

func TestRunRejectsRenderedPayloadBeforeNetwork(t *testing.T) {
	userYAML := strings.Replace(validUserYAML, "question: \"Who's in on {end_date}?\"", "question: \"Who's in on {weekday}?\"", 1)
	files := writeConfigs(t, userYAML, validSystemYAML)
	var logs bytes.Buffer
	client := &fakeDiscordClient{}
	dependencies := testDependencies(t, files, &logs, client)

	code := run(context.Background(), []string{
		"--config", files.user,
		"--system-config", files.system,
		"--dry-run",
	}, dependencies)
	if code != exitConfiguration {
		t.Errorf("run() = %d, want %d; logs: %s", code, exitConfiguration, logs.String())
	}
	if client.validateCalls != 0 || client.createCalls != 0 {
		t.Errorf("invalid payload made network calls: GET %d, POST %d", client.validateCalls, client.createCalls)
	}
}

func TestRunHelpDoesNotRequireConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	dependencies := cliDependencies{stdout: &stdout, stderr: &stderr}
	code := run(context.Background(), []string{"--help"}, dependencies)
	if code != exitSuccess {
		t.Errorf("run() = %d, want %d", code, exitSuccess)
	}
	if !strings.Contains(stdout.String(), "--system-config") {
		t.Errorf("help missing system config flag: %s", stdout.String())
	}
}

type configPaths struct {
	user   string
	system string
}

type fakeDiscordClient struct {
	webhook       *discord.Webhook
	message       *discord.CreatedMessage
	validateErr   error
	createErr     error
	validateCalls int
	createCalls   int
	webhookURL    string
	payload       discord.Payload
}

func (client *fakeDiscordClient) ValidateWebhook(_ context.Context, webhookURL string) (*discord.Webhook, error) {
	client.validateCalls++
	client.webhookURL = webhookURL
	if client.validateErr != nil {
		return nil, client.validateErr
	}
	return client.webhook, nil
}

func (client *fakeDiscordClient) CreatePollWithPayload(_ context.Context, _ string, buildPayload func() (discord.Payload, error)) (*discord.CreatedMessage, error) {
	client.createCalls++
	payload, err := buildPayload()
	if err != nil {
		return nil, err
	}
	client.payload = payload
	if client.createErr != nil {
		return nil, client.createErr
	}
	return client.message, nil
}

func writeConfigs(t *testing.T, user, system string) configPaths {
	t.Helper()
	directory := t.TempDir()
	userPath := filepath.Join(directory, "user-config.yaml")
	systemPath := filepath.Join(directory, "system-config.yaml")
	if err := os.WriteFile(userPath, []byte(user), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(systemPath, []byte(system), 0600); err != nil {
		t.Fatal(err)
	}
	return configPaths{user: userPath, system: systemPath}
}

func testDependencies(t *testing.T, files configPaths, logs *bytes.Buffer, client discordClient) cliDependencies {
	t.Helper()
	loggerFactory := func(level string) (*zap.Logger, error) {
		parsedLevel, err := zapcore.ParseLevel(level)
		if err != nil {
			return nil, err
		}
		core := zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(logs), parsedLevel)
		return zap.New(core), nil
	}
	return cliDependencies{
		now: func() time.Time {
			location, err := time.LoadLocation("Europe/Berlin")
			if err != nil {
				t.Fatal(err)
			}
			return time.Date(2026, time.October, 5, 12, 0, 0, 0, location)
		},
		loggerFactory: loggerFactory,
		clientFactory: func(*http.Client, int, time.Duration) (discordClient, error) {
			if client == nil {
				return nil, errors.New("fake client missing")
			}
			return client, nil
		},
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
}

const validUserYAML = `webhook_url: https://discord.com/api/webhooks/123456789/secret-token
target:
  timezone: Europe/Berlin
  weekday: Thursday
  time: "18:00"
  same_day_threshold: 3h
poll:
  question: "Who's in on {end_date}?"
  date_format: "02.01.2006"
  allow_multiselect: false
  answers:
    - text: "I'm in on {end_date}"
      emoji: "✅"
`

const validSystemYAML = `logging:
  level: info
http:
  request_timeout: 10s
  max_retries: 2
  max_rate_limit_wait: 60s
`
