package config

import "time"

// UserConfig represents the user-supplied configuration for the poll.
type UserConfig struct {
	WebhookURLEnv string       `yaml:"webhook_url_env"`
	Target        TargetConfig `yaml:"target"`
	Poll          PollConfig   `yaml:"poll"`
}

// TargetConfig specifies the target schedule.
type TargetConfig struct {
	Timezone         string        `yaml:"timezone"`
	Weekday          string        `yaml:"weekday"`
	Time             string        `yaml:"time"`
	SameDayThreshold time.Duration `yaml:"same_day_threshold"`
}

// PollConfig specifies the poll content.
type PollConfig struct {
	Question         string         `yaml:"question"`
	DateFormat       string         `yaml:"date_format"`
	AllowMultiselect bool           `yaml:"allow_multiselect"`
	Answers          []AnswerConfig `yaml:"answers"`
}

// AnswerConfig represents a single poll answer.
type AnswerConfig struct {
	Text  string `yaml:"text"`
	Emoji string `yaml:"emoji,omitempty"`
}

// SystemConfig represents operator-controlled logging and HTTP behavior.
type SystemConfig struct {
	Logging LoggingConfig `yaml:"logging"`
	HTTP    HTTPConfig    `yaml:"http"`
}

// LoggingConfig specifies logging behavior.
type LoggingConfig struct {
	Level string `yaml:"level"`
}

// HTTPConfig specifies HTTP client behavior.
type HTTPConfig struct {
	RequestTimeout   time.Duration `yaml:"request_timeout"`
	MaxRetries       int           `yaml:"max_retries"`
	MaxRateLimitWait time.Duration `yaml:"max_rate_limit_wait"`
}
