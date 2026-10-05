package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPayload(t *testing.T) {
	tests := []struct {
		name      string
		question  string
		answers   []Answer
		duration  int
		wantError bool
	}{
		{"valid single answer", "Poll question", []Answer{{Text: "Yes"}}, 24, false},
		{"question limit counts Unicode code points", strings.Repeat("🌍", 300), []Answer{{Text: "Yes"}}, 1, false},
		{"question grapheme sequences count constituent code points", strings.Repeat("👨‍👩‍👧‍👦", 43), []Answer{{Text: "Yes"}}, 24, true},
		{"answer limit counts Unicode code points", "Question", []Answer{{Text: strings.Repeat("é", 55)}}, 768, false},
		{"answer grapheme sequences count constituent code points", "Question", []Answer{{Text: strings.Repeat("👨‍👩‍👧‍👦", 8)}}, 24, true},
		{"empty question", " \t ", []Answer{{Text: "Yes"}}, 24, true},
		{"question exceeds limit", strings.Repeat("é", 301), []Answer{{Text: "Yes"}}, 24, true},
		{"no answers", "Question", nil, 24, true},
		{"too many answers", "Question", makeAnswers(11, "Answer"), 24, true},
		{"empty answer", "Question", []Answer{{Text: " \t "}}, 24, true},
		{"answer exceeds limit", "Question", []Answer{{Text: strings.Repeat("a", 56)}}, 24, true},
		{"duration below minimum", "Question", []Answer{{Text: "Yes"}}, 0, true},
		{"duration above maximum", "Question", []Answer{{Text: "Yes"}}, 769, true},
		{"invalid UTF-8", string([]byte{0xff}), []Answer{{Text: "Yes"}}, 24, true},
		{"invalid custom emoji", "Question", []Answer{{Text: "Yes", Emoji: "<:party:not-an-id>"}}, 24, true},
		{"plain text emoji", "Question", []Answer{{Text: "Yes", Emoji: "not an emoji"}}, 24, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload, err := BuildPayload(test.question, test.answers, test.duration, true)
			if test.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.question, payload.Poll.Question.Text)
			assert.Equal(t, test.duration, payload.Poll.Duration)
			assert.True(t, payload.Poll.AllowMultiselect)
			assert.Len(t, payload.Poll.Answers, len(test.answers))
		})
	}
}

func TestBuildPayloadEmojiMappingAndJSON(t *testing.T) {
	payload, err := BuildPayload("Question", []Answer{
		{Text: "Unicode", Emoji: "👨‍👩‍👧‍👦"},
		{Text: "Custom", Emoji: "<:party:123456789012345678>"},
		{Text: "Animated custom", Emoji: "<a:party:123456789012345678>"},
		{Text: "No emoji"},
	}, 72, false)
	require.NoError(t, err)

	data, err := json.Marshal(payload)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"poll": {
			"question": {"text":"Question"},
			"answers": [
				{"poll_media":{"text":"Unicode","emoji":{"name":"👨‍👩‍👧‍👦"}}},
				{"poll_media":{"text":"Custom","emoji":{"id":"123456789012345678"}}},
				{"poll_media":{"text":"Animated custom","emoji":{"id":"123456789012345678"}}},
				{"poll_media":{"text":"No emoji"}}
			],
			"duration":72,
			"allow_multiselect":false
		}
	}`, string(data))
}

func TestCreatePoll(t *testing.T) {
	payload := validPayload(t)
	var requestCount atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/webhooks/123456789/secret-token", r.URL.Path)
		assert.Equal(t, "true", r.URL.Query().Get("wait"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var got Payload
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, payload, got)
		_, _ = fmt.Fprint(w, `{"id":"987654321","poll":{"expiry":"2026-10-08T17:59:59Z"}}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.Client(), 0, time.Second)
	message, err := client.CreatePoll(context.Background(), webhookURL(server.URL), payload)
	require.NoError(t, err)
	require.NotNil(t, message)
	assert.Equal(t, "987654321", message.ID)
	require.NotNil(t, message.Poll)
	require.NotNil(t, message.Poll.Expiry)
	assert.Equal(t, "2026-10-08T17:59:59Z", message.Poll.Expiry.Format(time.RFC3339))
	assert.Equal(t, int32(1), requestCount.Load())
}

func TestCreatePollRejectsInvalidPayloadBeforeNetwork(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requestCount.Add(1)
	}))
	defer server.Close()

	client := newTestClient(t, server.Client(), 0, time.Second)
	_, err := client.CreatePoll(context.Background(), webhookURL(server.URL), Payload{})
	require.Error(t, err)
	assert.Zero(t, requestCount.Load())
}

func TestCreatePollAllowsMissingExpiry(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"id":"987654321","poll":{"expiry":null}}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.Client(), 0, time.Second)
	message, err := client.CreatePoll(context.Background(), webhookURL(server.URL), validPayload(t))
	require.NoError(t, err)
	require.NotNil(t, message)
	assert.Equal(t, "987654321", message.ID)
	assert.Nil(t, message.Poll.Expiry)
}

func TestValidateWebhook(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/webhooks/123456789/secret-token", r.URL.Path)
		_, _ = fmt.Fprint(w, `{"id":"123456789"}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.Client(), 0, time.Second)
	webhook, err := client.ValidateWebhook(context.Background(), webhookURL(server.URL))
	require.NoError(t, err)
	assert.Equal(t, "123456789", webhook.ID)
	assert.Equal(t, int32(1), requestCount.Load())
}

func TestValidateWebhookRejectsInvalidURLs(t *testing.T) {
	for _, rawURL := range []string{
		"http://discord.example/api/webhooks/123/token",
		"https://discord.example/",
		"https://user:pass@discord.example/api/webhooks/123/token",
		"https://discord.example/api/webhooks/123/token?wait=true",
		"https://discord.example/api/webhooks/123/token#fragment",
		"https://discord.example/api/webhooks/not-numeric/token",
		"https://evil.example/api/webhooks/123/token",
		"https://discord.com:8443/api/webhooks/123/token",
		"https://discord.com/api/vnope/webhooks/123/token",
	} {
		t.Run(rawURL, func(t *testing.T) {
			err := ValidateWebhookURL(rawURL)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "token")
		})
	}
}

func TestValidateWebhookURLAcceptsDiscordRoutes(t *testing.T) {
	for _, rawURL := range []string{
		"https://discord.com/api/webhooks/123456789/token",
		"https://discord.com/api/v10/webhooks/123456789/token",
		"https://discordapp.com/api/webhooks/123456789/token",
	} {
		assert.NoError(t, ValidateWebhookURL(rawURL))
	}
}

func TestNewClientRequiresExplicitHTTPTimeout(t *testing.T) {
	for _, httpClient := range []*http.Client{nil, {}} {
		_, err := NewClient(httpClient, 1, time.Second)
		require.Error(t, err)
	}
}

func TestValidatePayloadRejectsQuestionEmojiAndInvalidAnswerEmoji(t *testing.T) {
	payload := validPayload(t)
	payload.Poll.Question.Emoji = &PollEmoji{Name: "✅"}
	require.Error(t, validatePayload(payload))

	payload = validPayload(t)
	payload.Poll.Answers[0].PollMedia.Emoji = &PollEmoji{ID: "123", Name: "✅"}
	require.Error(t, validatePayload(payload))
}

func TestValidateWebhookAPIRejectionIsRedacted(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "invalid webhook token", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := newTestClient(t, server.Client(), 0, time.Second)
	_, err := client.ValidateWebhook(context.Background(), webhookURL(server.URL))
	require.ErrorIs(t, err, ErrAPI)
	assert.NotContains(t, err.Error(), "secret-token")
	assert.NotContains(t, err.Error(), server.URL)
}

func TestRateLimitRetry(t *testing.T) {
	tests := []struct {
		name       string
		retryAfter string
		retries    int
		wait       time.Duration
		wantCalls  int32
		wantErr    error
	}{
		{"retry and succeed", "0", 1, time.Second, 2, nil},
		{"retry count exhausted", "0", 0, time.Second, 1, ErrAPI},
		{"missing retry-after", "", 2, time.Second, 1, ErrAPI},
		{"invalid retry-after", "soon", 2, time.Second, 1, ErrAPI},
		{"negative retry-after", "-1", 2, time.Second, 1, ErrAPI},
		{"retry-after exceeds budget", "2", 2, time.Second, 1, ErrAPI},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var requestCount atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if requestCount.Add(1) == 1 {
					if test.retryAfter != "" {
						w.Header().Set("Retry-After", test.retryAfter)
					}
					w.WriteHeader(http.StatusTooManyRequests)
					_, _ = fmt.Fprint(w, `{"retry_after":0}`)
					return
				}
				_, _ = fmt.Fprint(w, `{"id":"987654321"}`)
			}))
			defer server.Close()

			client := newTestClient(t, server.Client(), test.retries, test.wait)
			message, err := client.CreatePoll(context.Background(), webhookURL(server.URL), validPayload(t))
			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				assert.Nil(t, message)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "987654321", message.ID)
			}
			assert.Equal(t, test.wantCalls, requestCount.Load())
		})
	}
}

func TestRateLimitWaitBudgetIsCumulative(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Retry-After", "0.6")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := newTestClient(t, server.Client(), 2, time.Second)
	_, err := client.CreatePoll(context.Background(), webhookURL(server.URL), validPayload(t))
	require.ErrorIs(t, err, ErrAPI)
	assert.Equal(t, int32(2), requestCount.Load())
}

func TestRateLimitAppliesToWebhookValidation(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requestCount.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = fmt.Fprint(w, `{"id":"123456789"}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.Client(), 1, time.Second)
	webhook, err := client.ValidateWebhook(context.Background(), webhookURL(server.URL))
	require.NoError(t, err)
	assert.Equal(t, "123456789", webhook.ID)
	assert.Equal(t, int32(2), requestCount.Load())
}

func TestRateLimitWaitHonorsContextCancellation(t *testing.T) {
	requestSeen := make(chan struct{}, 1)
	var requestCount atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		requestSeen <- struct{}{}
	}))
	defer server.Close()

	payload := validPayload(t)
	client := newTestClient(t, server.Client(), 2, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := client.CreatePoll(ctx, webhookURL(server.URL), payload)
		result <- err
	}()
	<-requestSeen
	cancel()

	err := <-result
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int32(1), requestCount.Load())
}

func TestServerErrorsAreNotRetried(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusBadGateway} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var requestCount atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requestCount.Add(1)
				http.Error(w, "server error", status)
			}))
			defer server.Close()

			client := newTestClient(t, server.Client(), 3, time.Second)
			_, err := client.CreatePoll(context.Background(), webhookURL(server.URL), validPayload(t))
			require.ErrorIs(t, err, ErrTransport)
			assert.Equal(t, int32(1), requestCount.Load())
		})
	}
}

func TestNetworkFailureIsNotRetriedAndRedacted(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		require.True(t, ok)
		conn, _, err := hijacker.Hijack()
		require.NoError(t, err)
		require.NoError(t, conn.Close())
	}))
	defer server.Close()

	client := newTestClient(t, server.Client(), 3, time.Second)
	_, err := client.CreatePoll(context.Background(), webhookURL(server.URL), validPayload(t))
	require.ErrorIs(t, err, ErrTransport)
	assert.NotContains(t, err.Error(), "secret-token")
	assert.NotContains(t, err.Error(), server.URL)
}

func TestClientTimeoutIsNotRetried(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		select {
		case <-r.Context().Done():
		case <-time.After(100 * time.Millisecond):
		}
	}))
	defer server.Close()

	httpClient := server.Client()
	httpClient.Timeout = 50 * time.Millisecond
	client, err := newClient(httpClient, 3, time.Second, true)
	require.NoError(t, err)

	_, err = client.CreatePoll(context.Background(), webhookURL(server.URL), validPayload(t))
	require.ErrorIs(t, err, ErrTransport)
	assert.Equal(t, int32(1), requestCount.Load())
	assert.NotContains(t, err.Error(), "secret-token")
}

func TestRedirectIsNotFollowed(t *testing.T) {
	var destinationCount atomic.Int32
	destination := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		destinationCount.Add(1)
	}))
	defer destination.Close()

	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", destination.URL)
		w.WriteHeader(http.StatusFound)
	}))
	defer source.Close()

	client := newTestClient(t, source.Client(), 0, time.Second)
	_, err := client.ValidateWebhook(context.Background(), webhookURL(source.URL))
	require.Error(t, err)
	assert.Zero(t, destinationCount.Load())
	assert.NotContains(t, err.Error(), "secret-token")
}

func TestAPIErrorDoesNotIncludeResponseBodyOrWebhookURL(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "failed request with secret-token", http.StatusBadRequest)
	}))
	defer server.Close()

	client := newTestClient(t, server.Client(), 0, time.Second)
	_, err := client.CreatePoll(context.Background(), webhookURL(server.URL), validPayload(t))
	require.ErrorIs(t, err, ErrAPI)
	assert.NotContains(t, err.Error(), "secret-token")
	assert.NotContains(t, err.Error(), server.URL)
}

func TestClientRejectsAPIResponseWithoutMessageID(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"poll":{"expiry":"2026-10-08T17:59:59Z"}}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.Client(), 0, time.Second)
	_, err := client.CreatePoll(context.Background(), webhookURL(server.URL), validPayload(t))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAPI) || errors.Is(err, ErrTransport))
}

func newTestClient(t *testing.T, httpClient *http.Client, maxRetries int, maxWait time.Duration) *Client {
	t.Helper()
	httpClient.Timeout = 5 * time.Second
	client, err := newClient(httpClient, maxRetries, maxWait, true)
	require.NoError(t, err)
	return client
}

func validPayload(t *testing.T) Payload {
	t.Helper()
	payload, err := BuildPayload("Question", []Answer{{Text: "Yes"}}, 24, false)
	require.NoError(t, err)
	return payload
}

func webhookURL(baseURL string) string {
	return baseURL + "/api/webhooks/123456789/secret-token"
}

func makeAnswers(count int, text string) []Answer {
	answers := make([]Answer, count)
	for index := range answers {
		answers[index].Text = text
	}
	return answers
}
