package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func validUserConfig() UserConfig {
	return UserConfig{
		WebhookURLEnv: "DISCORD_WEBHOOK_URL",
		Target: TargetConfig{
			Timezone:         "Europe/Berlin",
			Weekday:          "Thursday",
			Time:             "18:00",
			SameDayThreshold: time.Hour,
		},
		Poll: PollConfig{
			Question:         "Who's in on {end_date}?",
			DateFormat:       "02.01.2006",
			AllowMultiselect: false,
			Answers:          []AnswerConfig{{Text: "I'm in"}},
		},
	}
}

func validSystemConfig() SystemConfig {
	return SystemConfig{
		Logging: LoggingConfig{Level: "info"},
		HTTP: HTTPConfig{
			RequestTimeout:   10 * time.Second,
			MaxRetries:       2,
			MaxRateLimitWait: time.Minute,
		},
	}
}

func TestValidateUserConfig(t *testing.T) {
	tests := []struct {
		name        string
		input       UserConfig
		wantErr     bool
		errContains string
	}{
		{name: "webhook env standard", input: func() UserConfig { c := validUserConfig(); c.WebhookURLEnv = "DISCORD_WEBHOOK_URL"; return c }()},
		{name: "webhook env custom", input: func() UserConfig { c := validUserConfig(); c.WebhookURLEnv = "MY_WEBHOOK"; return c }()},
		{name: "webhook env with digits", input: func() UserConfig { c := validUserConfig(); c.WebhookURLEnv = "WEBHOOK_123"; return c }()},
		{name: "webhook env empty", input: func() UserConfig { c := validUserConfig(); c.WebhookURLEnv = ""; return c }(), wantErr: true, errContains: "webhook_url_env"},
		{name: "webhook env lowercase", input: func() UserConfig { c := validUserConfig(); c.WebhookURLEnv = "webhook_url"; return c }(), wantErr: true, errContains: "webhook_url_env"},
		{name: "webhook env special characters", input: func() UserConfig { c := validUserConfig(); c.WebhookURLEnv = "WEBHOOK-URL"; return c }(), wantErr: true, errContains: "webhook_url_env"},
		{name: "webhook env starts with number", input: func() UserConfig { c := validUserConfig(); c.WebhookURLEnv = "0WEBHOOK"; return c }(), wantErr: true, errContains: "webhook_url_env"},

		{name: "timezone Europe Berlin", input: func() UserConfig { c := validUserConfig(); c.Target.Timezone = "Europe/Berlin"; return c }()},
		{name: "timezone UTC", input: func() UserConfig { c := validUserConfig(); c.Target.Timezone = "UTC"; return c }()},
		{name: "timezone New York", input: func() UserConfig { c := validUserConfig(); c.Target.Timezone = "America/New_York"; return c }()},
		{name: "timezone empty", input: func() UserConfig { c := validUserConfig(); c.Target.Timezone = ""; return c }(), wantErr: true, errContains: "target.timezone"},
		{name: "timezone unknown", input: func() UserConfig { c := validUserConfig(); c.Target.Timezone = "InvalidTimezone"; return c }(), wantErr: true, errContains: "target.timezone"},
		{name: "timezone case sensitive", input: func() UserConfig { c := validUserConfig(); c.Target.Timezone = "europe/berlin"; return c }(), wantErr: true, errContains: "target.timezone"},

		{name: "weekday Monday", input: func() UserConfig { c := validUserConfig(); c.Target.Weekday = "Monday"; return c }()},
		{name: "weekday Tuesday", input: func() UserConfig { c := validUserConfig(); c.Target.Weekday = "Tuesday"; return c }()},
		{name: "weekday Wednesday", input: func() UserConfig { c := validUserConfig(); c.Target.Weekday = "Wednesday"; return c }()},
		{name: "weekday Thursday", input: func() UserConfig { c := validUserConfig(); c.Target.Weekday = "Thursday"; return c }()},
		{name: "weekday Friday", input: func() UserConfig { c := validUserConfig(); c.Target.Weekday = "Friday"; return c }()},
		{name: "weekday Saturday", input: func() UserConfig { c := validUserConfig(); c.Target.Weekday = "Saturday"; return c }()},
		{name: "weekday Sunday", input: func() UserConfig { c := validUserConfig(); c.Target.Weekday = "Sunday"; return c }()},
		{name: "weekday empty", input: func() UserConfig { c := validUserConfig(); c.Target.Weekday = ""; return c }(), wantErr: true, errContains: "target.weekday"},
		{name: "weekday lowercase", input: func() UserConfig { c := validUserConfig(); c.Target.Weekday = "monday"; return c }(), wantErr: true, errContains: "target.weekday"},
		{name: "weekday abbreviated", input: func() UserConfig { c := validUserConfig(); c.Target.Weekday = "Mon"; return c }(), wantErr: true, errContains: "target.weekday"},
		{name: "weekday unknown", input: func() UserConfig { c := validUserConfig(); c.Target.Weekday = "InvalidDay"; return c }(), wantErr: true, errContains: "target.weekday"},

		{name: "time evening", input: func() UserConfig { c := validUserConfig(); c.Target.Time = "18:00"; return c }()},
		{name: "time midnight", input: func() UserConfig { c := validUserConfig(); c.Target.Time = "00:00"; return c }()},
		{name: "time last minute", input: func() UserConfig { c := validUserConfig(); c.Target.Time = "23:59"; return c }()},
		{name: "time morning", input: func() UserConfig { c := validUserConfig(); c.Target.Time = "09:30"; return c }()},
		{name: "time empty", input: func() UserConfig { c := validUserConfig(); c.Target.Time = ""; return c }(), wantErr: true, errContains: "target.time"},
		{name: "time missing minutes", input: func() UserConfig { c := validUserConfig(); c.Target.Time = "18"; return c }(), wantErr: true, errContains: "target.time"},
		{name: "time includes seconds", input: func() UserConfig { c := validUserConfig(); c.Target.Time = "18:00:00"; return c }(), wantErr: true, errContains: "target.time"},
		{name: "time hour out of range", input: func() UserConfig { c := validUserConfig(); c.Target.Time = "25:00"; return c }(), wantErr: true, errContains: "target.time"},
		{name: "time minute out of range", input: func() UserConfig { c := validUserConfig(); c.Target.Time = "18:60"; return c }(), wantErr: true, errContains: "target.time"},
		{name: "time missing colon", input: func() UserConfig { c := validUserConfig(); c.Target.Time = "1800"; return c }(), wantErr: true, errContains: "target.time"},

		{name: "threshold one hour", input: func() UserConfig { c := validUserConfig(); c.Target.SameDayThreshold = time.Hour; return c }()},
		{name: "threshold fractional hour", input: func() UserConfig { c := validUserConfig(); c.Target.SameDayThreshold = 90 * time.Minute; return c }()},
		{name: "threshold three hours", input: func() UserConfig { c := validUserConfig(); c.Target.SameDayThreshold = 3 * time.Hour; return c }()},
		{name: "threshold twenty four hours", input: func() UserConfig { c := validUserConfig(); c.Target.SameDayThreshold = 24 * time.Hour; return c }()},
		{name: "threshold zero", input: func() UserConfig { c := validUserConfig(); c.Target.SameDayThreshold = 0; return c }(), wantErr: true, errContains: "target.same_day_threshold"},
		{name: "threshold fifty nine minutes", input: func() UserConfig { c := validUserConfig(); c.Target.SameDayThreshold = 59 * time.Minute; return c }(), wantErr: true, errContains: "target.same_day_threshold"},
		{name: "threshold thirty minutes", input: func() UserConfig { c := validUserConfig(); c.Target.SameDayThreshold = 30 * time.Minute; return c }(), wantErr: true, errContains: "target.same_day_threshold"},

		{name: "question with placeholder", input: func() UserConfig { c := validUserConfig(); c.Poll.Question = "Who's in on {end_date}?"; return c }()},
		{name: "question without placeholder", input: func() UserConfig { c := validUserConfig(); c.Poll.Question = "Poll question"; return c }()},
		{name: "question empty", input: func() UserConfig { c := validUserConfig(); c.Poll.Question = ""; return c }(), wantErr: true, errContains: "poll.question"},

		{name: "date format dotted", input: func() UserConfig { c := validUserConfig(); c.Poll.DateFormat = "02.01.2006"; return c }()},
		{name: "date format ISO", input: func() UserConfig { c := validUserConfig(); c.Poll.DateFormat = "2006-01-02"; return c }()},
		{name: "date format long weekday", input: func() UserConfig { c := validUserConfig(); c.Poll.DateFormat = "Monday, 02 January 2006"; return c }()},
		{name: "date format month", input: func() UserConfig { c := validUserConfig(); c.Poll.DateFormat = "Jan"; return c }()},
		{name: "date format empty", input: func() UserConfig { c := validUserConfig(); c.Poll.DateFormat = ""; return c }(), wantErr: true, errContains: "poll.date_format"},
		{name: "date format malformed Go layout", input: func() UserConfig { c := validUserConfig(); c.Poll.DateFormat = "DD.MM.YYYY"; return c }(), wantErr: true, errContains: "poll.date_format"},

		{name: "multiselect enabled", input: func() UserConfig { c := validUserConfig(); c.Poll.AllowMultiselect = true; return c }()},
		{name: "multiselect disabled", input: func() UserConfig { c := validUserConfig(); c.Poll.AllowMultiselect = false; return c }()},

		{name: "one answer", input: func() UserConfig { c := validUserConfig(); c.Poll.Answers = []AnswerConfig{{Text: "I'm in"}}; return c }()},
		{name: "ten answers", input: func() UserConfig {
			c := validUserConfig()
			c.Poll.Answers = make([]AnswerConfig, 10)
			for i := range c.Poll.Answers {
				c.Poll.Answers[i].Text = "Answer"
			}
			return c
		}()},
		{name: "no answers", input: func() UserConfig { c := validUserConfig(); c.Poll.Answers = []AnswerConfig{}; return c }(), wantErr: true, errContains: "poll.answers"},
		{name: "eleven answers", input: func() UserConfig {
			c := validUserConfig()
			c.Poll.Answers = make([]AnswerConfig, 11)
			for i := range c.Poll.Answers {
				c.Poll.Answers[i].Text = "Answer"
			}
			return c
		}(), wantErr: true, errContains: "poll.answers"},
		{name: "nil answers", input: func() UserConfig { c := validUserConfig(); c.Poll.Answers = nil; return c }(), wantErr: true, errContains: "poll.answers"},

		{name: "answer text ordinary", input: func() UserConfig { c := validUserConfig(); c.Poll.Answers[0].Text = "I'm in"; return c }()},
		{name: "answer text maybe", input: func() UserConfig { c := validUserConfig(); c.Poll.Answers[0].Text = "Maybe"; return c }()},
		{name: "answer text arbitrary", input: func() UserConfig { c := validUserConfig(); c.Poll.Answers[0].Text = "Any non-empty answer"; return c }()},
		{name: "answer text empty", input: func() UserConfig { c := validUserConfig(); c.Poll.Answers[0].Text = ""; return c }(), wantErr: true, errContains: "poll.answers"},

		{name: "emoji omitted", input: func() UserConfig { c := validUserConfig(); c.Poll.Answers[0].Emoji = ""; return c }()},
		{name: "emoji check", input: func() UserConfig { c := validUserConfig(); c.Poll.Answers[0].Emoji = "✅"; return c }()},
		{name: "emoji cross", input: func() UserConfig { c := validUserConfig(); c.Poll.Answers[0].Emoji = "❌"; return c }()},
		{name: "emoji celebration", input: func() UserConfig { c := validUserConfig(); c.Poll.Answers[0].Emoji = "🎉"; return c }()},
		{name: "emoji custom", input: func() UserConfig { c := validUserConfig(); c.Poll.Answers[0].Emoji = "<:party:123456789>"; return c }()},
		{name: "emoji animated custom", input: func() UserConfig { c := validUserConfig(); c.Poll.Answers[0].Emoji = "<a:party:123456789>"; return c }()},
		{name: "emoji malformed custom id", input: func() UserConfig { c := validUserConfig(); c.Poll.Answers[0].Emoji = "<:name:abc>"; return c }(), wantErr: true, errContains: "emoji"},
		{name: "emoji malformed custom format", input: func() UserConfig { c := validUserConfig(); c.Poll.Answers[0].Emoji = "<:name>"; return c }(), wantErr: true, errContains: "emoji"},
		{name: "emoji quote", input: func() UserConfig { c := validUserConfig(); c.Poll.Answers[0].Emoji = "\""; return c }(), wantErr: true, errContains: "emoji"},
		{name: "emoji plain text", input: func() UserConfig { c := validUserConfig(); c.Poll.Answers[0].Emoji = "not an emoji"; return c }(), wantErr: true, errContains: "emoji"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUserConfig(&tt.input)
			if tt.wantErr {
				assert.ErrorContains(t, err, tt.errContains)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestValidateSystemConfig(t *testing.T) {
	tests := []struct {
		name        string
		input       SystemConfig
		wantErr     bool
		errContains string
	}{
		{name: "debug logging", input: func() SystemConfig { c := validSystemConfig(); c.Logging.Level = "debug"; return c }()},
		{name: "info logging", input: func() SystemConfig { c := validSystemConfig(); c.Logging.Level = "info"; return c }()},
		{name: "warn logging", input: func() SystemConfig { c := validSystemConfig(); c.Logging.Level = "warn"; return c }()},
		{name: "error logging", input: func() SystemConfig { c := validSystemConfig(); c.Logging.Level = "error"; return c }()},
		{name: "empty logging level", input: func() SystemConfig { c := validSystemConfig(); c.Logging.Level = ""; return c }(), wantErr: true, errContains: "logging.level"},
		{name: "unknown logging level", input: func() SystemConfig { c := validSystemConfig(); c.Logging.Level = "trace"; return c }(), wantErr: true, errContains: "logging.level"},

		{name: "request timeout minimum", input: func() SystemConfig { c := validSystemConfig(); c.HTTP.RequestTimeout = time.Second; return c }()},
		{name: "request timeout maximum", input: func() SystemConfig { c := validSystemConfig(); c.HTTP.RequestTimeout = 60 * time.Second; return c }()},
		{name: "request timeout zero", input: func() SystemConfig { c := validSystemConfig(); c.HTTP.RequestTimeout = 0; return c }(), wantErr: true, errContains: "http.request_timeout"},
		{name: "request timeout below minimum", input: func() SystemConfig { c := validSystemConfig(); c.HTTP.RequestTimeout = time.Second - 1; return c }(), wantErr: true, errContains: "http.request_timeout"},
		{name: "request timeout above maximum", input: func() SystemConfig { c := validSystemConfig(); c.HTTP.RequestTimeout = 61 * time.Second; return c }(), wantErr: true, errContains: "http.request_timeout"},

		{name: "retries minimum", input: func() SystemConfig { c := validSystemConfig(); c.HTTP.MaxRetries = 0; return c }()},
		{name: "retries maximum", input: func() SystemConfig { c := validSystemConfig(); c.HTTP.MaxRetries = 5; return c }()},
		{name: "retries negative", input: func() SystemConfig { c := validSystemConfig(); c.HTTP.MaxRetries = -1; return c }(), wantErr: true, errContains: "http.max_retries"},
		{name: "retries above maximum", input: func() SystemConfig { c := validSystemConfig(); c.HTTP.MaxRetries = 6; return c }(), wantErr: true, errContains: "http.max_retries"},

		{name: "rate limit wait minimum", input: func() SystemConfig { c := validSystemConfig(); c.HTTP.MaxRateLimitWait = time.Second; return c }()},
		{name: "rate limit wait maximum", input: func() SystemConfig { c := validSystemConfig(); c.HTTP.MaxRateLimitWait = 120 * time.Second; return c }()},
		{name: "rate limit wait zero", input: func() SystemConfig { c := validSystemConfig(); c.HTTP.MaxRateLimitWait = 0; return c }(), wantErr: true, errContains: "http.max_rate_limit_wait"},
		{name: "rate limit wait below minimum", input: func() SystemConfig { c := validSystemConfig(); c.HTTP.MaxRateLimitWait = time.Second - 1; return c }(), wantErr: true, errContains: "http.max_rate_limit_wait"},
		{name: "rate limit wait above maximum", input: func() SystemConfig { c := validSystemConfig(); c.HTTP.MaxRateLimitWait = 121 * time.Second; return c }(), wantErr: true, errContains: "http.max_rate_limit_wait"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSystemConfig(&tt.input)
			if tt.wantErr {
				assert.ErrorContains(t, err, tt.errContains)
				return
			}
			assert.NoError(t, err)
		})
	}
}
