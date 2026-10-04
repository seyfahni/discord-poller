package config

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	webhookURLEnvPattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	customEmojiPattern   = regexp.MustCompile(`^<a?:\w+:\d+>$`)
)

func ValidateUserConfig(cfg *UserConfig) error {
	if cfg == nil {
		return fmt.Errorf("user config must not be nil")
	}
	if err := validateWebhookURLEnv(cfg.WebhookURLEnv); err != nil {
		return err
	}
	if err := validateTargetConfig(cfg.Target); err != nil {
		return err
	}
	return validatePollConfig(cfg.Poll)
}

func ValidateSystemConfig(cfg *SystemConfig) error {
	if cfg == nil {
		return fmt.Errorf("system config must not be nil")
	}
	if err := validateLoggingConfig(cfg.Logging); err != nil {
		return err
	}
	return validateHTTPConfig(cfg.HTTP)
}

func validateWebhookURLEnv(env string) error {
	if env == "" || !webhookURLEnvPattern.MatchString(env) {
		return fmt.Errorf("webhook_url_env must be a non-empty uppercase environment variable name matching %s", webhookURLEnvPattern)
	}
	return nil
}

func validateTargetConfig(cfg TargetConfig) error {
	if err := validateTimezone(cfg.Timezone); err != nil {
		return fmt.Errorf("target.timezone: %w", err)
	}
	if err := validateWeekday(cfg.Weekday); err != nil {
		return fmt.Errorf("target.weekday: %w", err)
	}
	if err := validateTime(cfg.Time); err != nil {
		return fmt.Errorf("target.time: %w", err)
	}
	if err := validateThreshold(cfg.SameDayThreshold); err != nil {
		return fmt.Errorf("target.same_day_threshold: %w", err)
	}
	return nil
}

func validateTimezone(tz string) error {
	if tz == "" {
		return fmt.Errorf("must not be empty")
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("must be a valid IANA timezone: %w", err)
	}
	return nil
}

func validateWeekday(weekday string) error {
	switch weekday {
	case "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday":
		return nil
	default:
		return fmt.Errorf("must be a weekday name from Monday through Sunday")
	}
}

func validateTime(timeStr string) error {
	if timeStr == "" {
		return fmt.Errorf("must not be empty")
	}
	if _, err := time.Parse("15:04", timeStr); err != nil {
		return fmt.Errorf("must be a 24-hour time in HH:MM format: %w", err)
	}
	return nil
}

func validateThreshold(threshold time.Duration) error {
	if threshold < time.Hour {
		return fmt.Errorf("must be at least 1 hour")
	}
	return nil
}

func validatePollConfig(cfg PollConfig) error {
	if err := validateQuestion(cfg.Question); err != nil {
		return fmt.Errorf("poll.question: %w", err)
	}
	if err := validateDateFormat(cfg.DateFormat); err != nil {
		return fmt.Errorf("poll.date_format: %w", err)
	}
	if err := validateAnswers(cfg.Answers); err != nil {
		return err
	}
	return nil
}

func validateQuestion(question string) error {
	if strings.TrimSpace(question) == "" {
		return fmt.Errorf("must not be empty")
	}
	return nil
}

func validateDateFormat(format string) error {
	if strings.TrimSpace(format) == "" {
		return fmt.Errorf("must not be empty")
	}
	formatted := time.Now().Format(format)
	if _, err := time.Parse(format, formatted); err != nil {
		return fmt.Errorf("must be a valid Go time layout: %w", err)
	}
	if formatted == format {
		return fmt.Errorf("must be a valid Go time layout containing date or time fields")
	}
	return nil
}

func validateAnswers(answers []AnswerConfig) error {
	if len(answers) == 0 {
		return fmt.Errorf("poll.answers must contain at least 1 answer")
	}
	if len(answers) > 10 {
		return fmt.Errorf("poll.answers must not contain more than 10 answers")
	}
	for i, answer := range answers {
		if err := validateAnswerText(answer.Text, i); err != nil {
			return err
		}
		if err := validateEmoji(answer.Emoji); err != nil {
			return fmt.Errorf("poll.answers[%d].emoji: %w", i, err)
		}
	}
	return nil
}

func validateAnswerText(text string, index int) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("poll.answers[%d].text must not be empty", index)
	}
	return nil
}

func validateEmoji(emoji string) error {
	if emoji == "" {
		return nil
	}
	if !utf8.ValidString(emoji) {
		return fmt.Errorf("invalid emoji %q: must be valid UTF-8", emoji)
	}
	if customEmojiPattern.MatchString(emoji) {
		return nil
	}
	hasSymbol := false
	for _, r := range emoji {
		if unicode.IsSymbol(r) {
			hasSymbol = true
			continue
		}
		if unicode.IsMark(r) || r == '\u200d' {
			continue
		}
		return fmt.Errorf("invalid emoji %q: must be a Unicode emoji or custom Discord emoji", emoji)
	}
	if !hasSymbol {
		return fmt.Errorf("invalid emoji %q: must be a Unicode emoji or custom Discord emoji", emoji)
	}
	return nil
}

func validateLoggingConfig(cfg LoggingConfig) error {
	if err := validateLogLevel(cfg.Level); err != nil {
		return fmt.Errorf("logging.level: %w", err)
	}
	return nil
}

func validateLogLevel(level string) error {
	switch level {
	case "debug", "info", "warn", "error":
		return nil
	default:
		return fmt.Errorf("must be one of debug, info, warn, or error")
	}
}

func validateHTTPConfig(cfg HTTPConfig) error {
	if err := validateRequestTimeout(cfg.RequestTimeout); err != nil {
		return fmt.Errorf("http.request_timeout: %w", err)
	}
	if err := validateMaxRetries(cfg.MaxRetries); err != nil {
		return fmt.Errorf("http.max_retries: %w", err)
	}
	if err := validateMaxRateLimitWait(cfg.MaxRateLimitWait); err != nil {
		return fmt.Errorf("http.max_rate_limit_wait: %w", err)
	}
	return nil
}

func validateRequestTimeout(timeout time.Duration) error {
	if timeout < time.Second || timeout > 60*time.Second {
		return fmt.Errorf("must be between 1 and 60 seconds")
	}
	return nil
}

func validateMaxRetries(retries int) error {
	if retries < 0 || retries > 5 {
		return fmt.Errorf("must be between 0 and 5")
	}
	return nil
}

func validateMaxRateLimitWait(wait time.Duration) error {
	if wait < time.Second || wait > 120*time.Second {
		return fmt.Errorf("must be between 1 and 120 seconds")
	}
	return nil
}
