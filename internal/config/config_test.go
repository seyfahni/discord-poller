package config

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestUserConfigYAML(t *testing.T) {
	data := `
webhook_url_env: DISCORD_WEBHOOK_URL
target:
  timezone: Europe/Berlin
  weekday: Thursday
  time: "18:00"
  same_day_threshold: 1.5h
poll:
  question: "Who's in on {end_date}?"
  date_format: "02.01.2006"
  allow_multiselect: true
  answers:
    - text: "I'm in"
      emoji: "✅"
    - text: "Maybe"
`
	var cfg UserConfig
	if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.WebhookURLEnv != "DISCORD_WEBHOOK_URL" {
		t.Errorf("unexpected webhook environment variable: %q", cfg.WebhookURLEnv)
	}
	wantTarget := TargetConfig{
		Timezone: "Europe/Berlin", Weekday: "Thursday", Time: "18:00",
		SameDayThreshold: 90 * time.Minute,
	}
	if cfg.Target != wantTarget {
		t.Errorf("target = %+v, want %+v", cfg.Target, wantTarget)
	}
	if cfg.Poll.Question != "Who's in on {end_date}?" ||
		cfg.Poll.DateFormat != "02.01.2006" || !cfg.Poll.AllowMultiselect {
		t.Errorf("unexpected poll content: %+v", cfg.Poll)
	}
	if len(cfg.Poll.Answers) != 2 {
		t.Fatalf("answers count = %d, want 2", len(cfg.Poll.Answers))
	}
	if cfg.Poll.Answers[0] != (AnswerConfig{Text: "I'm in", Emoji: "✅"}) ||
		cfg.Poll.Answers[1] != (AnswerConfig{Text: "Maybe"}) {
		t.Errorf("unexpected answers: %+v", cfg.Poll.Answers)
	}
}

func TestSystemConfigYAML(t *testing.T) {
	data := `
logging:
  level: info
http:
  request_timeout: 10s
  max_retries: 2
  max_rate_limit_wait: 60s
`
	var cfg SystemConfig
	if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
		t.Fatal(err)
	}
	want := SystemConfig{
		Logging: LoggingConfig{Level: "info"},
		HTTP: HTTPConfig{
			RequestTimeout: 10 * time.Second,
			MaxRetries:     2, MaxRateLimitWait: time.Minute,
		},
	}
	if cfg != want {
		t.Errorf("system config = %+v, want %+v", cfg, want)
	}
}

func TestAnswerConfigYAML(t *testing.T) {
	for _, tt := range []struct {
		name   string
		answer AnswerConfig
		want   string
	}{
		{name: "without emoji", answer: AnswerConfig{Text: "Maybe"}, want: "text: Maybe\n"},
		{name: "with emoji", answer: AnswerConfig{Text: "Yes", Emoji: "✅"}, want: "text: \"Yes\"\nemoji: ✅\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data, err := yaml.Marshal(tt.answer)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != tt.want {
				t.Errorf("YAML = %q, want %q", data, tt.want)
			}
		})
	}
}
