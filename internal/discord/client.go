// Package discord builds poll payloads and executes Discord webhook requests.
package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const maxResponseBytes = 1 << 20

var webhookTokenPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

var (
	// ErrConfiguration identifies invalid local webhook or payload input.
	ErrConfiguration = errors.New("invalid discord request configuration")
	// ErrTransport identifies failures with an ambiguous or unavailable response.
	ErrTransport = errors.New("discord transport failure")
	// ErrAPI identifies Discord API rejections, including exhausted rate limits.
	ErrAPI = errors.New("discord API rejection")
)

// ClientError reports a categorized Discord request failure without exposing
// webhook credentials or response bodies.
type ClientError struct {
	kind       error
	operation  string
	statusCode int
	cause      error
}

func (err *ClientError) Error() string {
	if err.statusCode != 0 {
		return fmt.Sprintf("discord %s failed with HTTP status %d", err.operation, err.statusCode)
	}
	if errors.Is(err.cause, context.Canceled) {
		return fmt.Sprintf("discord %s was canceled", err.operation)
	}
	if errors.Is(err.cause, context.DeadlineExceeded) {
		return fmt.Sprintf("discord %s timed out", err.operation)
	}
	return fmt.Sprintf("discord %s failed: %s", err.operation, err.kind)
}

// Is supports classification while keeping the underlying transport error
// out of the printable error chain.
func (err *ClientError) Is(target error) bool {
	if target == nil {
		return false
	}
	return target == err.kind || errors.Is(err.cause, target)
}

// Client sends native poll requests to Discord webhooks.
type Client struct {
	httpClient       *http.Client
	maxRetries       int
	maxRateLimitWait time.Duration
	allowCustomHost  bool
}

// NewClient creates a Discord client with a bounded HTTP 429 retry policy.
// The provided HTTP client must have an explicit timeout.
func NewClient(httpClient *http.Client, maxRetries int, maxRateLimitWait time.Duration) (*Client, error) {
	return newClient(httpClient, maxRetries, maxRateLimitWait, false)
}

func newClient(httpClient *http.Client, maxRetries int, maxRateLimitWait time.Duration, allowCustomHost bool) (*Client, error) {
	if maxRetries < 0 || maxRetries > 5 {
		return nil, fmt.Errorf("maximum retries must be between 0 and 5")
	}
	if maxRateLimitWait < time.Second || maxRateLimitWait > 120*time.Second {
		return nil, fmt.Errorf("maximum rate-limit wait must be between 1 and 120 seconds")
	}
	if httpClient == nil {
		return nil, fmt.Errorf("HTTP client with an explicit timeout is required")
	}
	if httpClient.Timeout <= 0 {
		return nil, fmt.Errorf("HTTP client with an explicit timeout is required")
	}

	safeClient := *httpClient
	safeClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &Client{
		httpClient:       &safeClient,
		maxRetries:       maxRetries,
		maxRateLimitWait: maxRateLimitWait,
		allowCustomHost:  allowCustomHost,
	}, nil
}

// ValidateWebhookURL checks that rawURL is an HTTPS webhook URL without
// including its credential-bearing value in any returned error.
func ValidateWebhookURL(rawURL string) error {
	_, err := parseWebhookURL(rawURL, false)
	return err
}

func parseWebhookURL(rawURL string, allowCustomHost bool) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("webhook URL must be an HTTPS Discord webhook URL")
	}
	if !allowCustomHost && parsed.Port() != "" && parsed.Port() != "443" {
		return nil, fmt.Errorf("webhook URL must be an HTTPS Discord webhook URL")
	}
	host := strings.ToLower(parsed.Hostname())
	if !allowCustomHost && host != "discord.com" && host != "discordapp.com" {
		return nil, fmt.Errorf("webhook URL must be an HTTPS Discord webhook URL")
	}

	segments := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	validPath := len(segments) == 4 &&
		segments[0] == "api" &&
		segments[1] == "webhooks"
	if len(segments) == 5 {
		validPath = segments[0] == "api" &&
			validAPIVersion(segments[1]) &&
			segments[2] == "webhooks"
		segments = []string{segments[0], segments[2], segments[3], segments[4]}
	}
	if !validPath || len(segments) != 4 ||
		segments[1] != "webhooks" ||
		!isNumeric(segments[2]) ||
		!webhookTokenPattern.MatchString(segments[3]) {
		return nil, fmt.Errorf("webhook URL must be an HTTPS Discord webhook URL")
	}
	return parsed, nil
}

// ValidateWebhook performs a non-mutating GET using the webhook token.
func (client *Client) ValidateWebhook(ctx context.Context, webhookURL string) (*Webhook, error) {
	if ctx == nil {
		return nil, clientError(ErrConfiguration, "webhook validation", 0, nil)
	}
	if _, err := parseWebhookURL(webhookURL, client.allowCustomHost); err != nil {
		return nil, clientError(ErrConfiguration, "webhook validation", 0, nil)
	}

	response, err := client.do(ctx, http.MethodGet, webhookURL, nil, "webhook validation")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, responseError("webhook validation", response.StatusCode)
	}
	data, err := readResponse(response.Body)
	if err != nil {
		return nil, clientError(ErrAPI, "webhook validation", response.StatusCode, err)
	}
	webhook, err := decodeWebhook(data)
	if err != nil {
		return nil, clientError(ErrAPI, "webhook validation", response.StatusCode, err)
	}
	return webhook, nil
}

// CreatePoll submits a validated poll and returns the created message response.
func (client *Client) CreatePoll(ctx context.Context, webhookURL string, payload Payload) (*CreatedMessage, error) {
	if ctx == nil {
		return nil, clientError(ErrConfiguration, "poll creation", 0, nil)
	}
	if _, err := parseWebhookURL(webhookURL, client.allowCustomHost); err != nil {
		return nil, clientError(ErrConfiguration, "poll creation", 0, nil)
	}
	if err := validatePayload(payload); err != nil {
		return nil, clientError(ErrConfiguration, "poll creation", 0, nil)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, clientError(ErrConfiguration, "poll creation", 0, err)
	}

	parsed, err := url.Parse(webhookURL)
	if err != nil {
		return nil, clientError(ErrConfiguration, "poll creation", 0, nil)
	}
	query := parsed.Query()
	query.Set("wait", "true")
	parsed.RawQuery = query.Encode()

	response, err := client.do(ctx, http.MethodPost, parsed.String(), body, "poll creation")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, responseError("poll creation", response.StatusCode)
	}
	data, err := readResponse(response.Body)
	if err != nil {
		return nil, clientError(ErrTransport, "poll creation", response.StatusCode, err)
	}
	message, err := decodeCreatedMessage(data)
	if err != nil {
		return nil, clientError(ErrTransport, "poll creation", response.StatusCode, err)
	}
	return message, nil
}

func (client *Client) do(ctx context.Context, method, endpoint string, body []byte, operation string) (*http.Response, error) {
	var waited time.Duration
	for attempt := 0; ; attempt++ {
		request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, clientError(ErrConfiguration, operation, 0, nil)
		}
		if method == http.MethodPost {
			request.Header.Set("Content-Type", "application/json")
		}
		request.Header.Set("Accept", "application/json")

		response, err := client.httpClient.Do(request)
		if err != nil {
			return nil, clientError(ErrTransport, operation, 0, err)
		}
		if response.StatusCode != http.StatusTooManyRequests {
			return response, nil
		}

		delay, err := parseRetryAfter(response.Header.Get("Retry-After"))
		closeResponse(response)
		if err != nil || attempt >= client.maxRetries || delay > client.maxRateLimitWait-waited {
			return nil, clientError(ErrAPI, operation, http.StatusTooManyRequests, nil)
		}
		waited += delay
		if err := waitContext(ctx, delay); err != nil {
			return nil, clientError(ErrTransport, operation, 0, err)
		}
	}
}

func responseError(operation string, statusCode int) error {
	kind := ErrAPI
	if statusCode >= http.StatusInternalServerError {
		kind = ErrTransport
	}
	return clientError(kind, operation, statusCode, nil)
}

func clientError(kind error, operation string, statusCode int, cause error) error {
	return &ClientError{
		kind:       kind,
		operation:  operation,
		statusCode: statusCode,
		cause:      cause,
	}
}

func parseRetryAfter(value string) (time.Duration, error) {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
		return 0, fmt.Errorf("invalid Retry-After value")
	}
	nanoseconds := math.Ceil(seconds * float64(time.Second))
	if nanoseconds > float64(math.MaxInt64) {
		return 0, fmt.Errorf("Retry-After value is too large")
	}
	return time.Duration(nanoseconds), nil
}

func waitContext(ctx context.Context, delay time.Duration) error {
	if delay == 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func closeResponse(response *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
	_ = response.Body.Close()
}

func readResponse(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read discord response")
	}
	if len(data) > maxResponseBytes {
		return nil, fmt.Errorf("discord response is too large")
	}
	return data, nil
}

func isNumeric(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validAPIVersion(value string) bool {
	if len(value) < 2 || value[0] != 'v' {
		return false
	}
	return isNumeric(value[1:])
}
