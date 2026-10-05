package discord

import (
	"encoding/json"
	"fmt"
	"time"
)

// Webhook represents the identifying data returned by Discord's webhook check.
type Webhook struct {
	ID string `json:"id"`
}

// CreatedMessage represents the created webhook message fields used by the poller.
type CreatedMessage struct {
	ID   string       `json:"id"`
	Poll *CreatedPoll `json:"poll,omitempty"`
}

// CreatedPoll contains the expiry returned for a created poll.
type CreatedPoll struct {
	Expiry *time.Time `json:"expiry"`
}

func decodeWebhook(data []byte) (*Webhook, error) {
	var webhook Webhook
	if err := json.Unmarshal(data, &webhook); err != nil || webhook.ID == "" {
		return nil, fmt.Errorf("discord returned an invalid webhook response")
	}
	return &webhook, nil
}

func decodeCreatedMessage(data []byte) (*CreatedMessage, error) {
	var message CreatedMessage
	if err := json.Unmarshal(data, &message); err != nil || message.ID == "" {
		return nil, fmt.Errorf("discord returned an invalid message response")
	}
	return &message, nil
}
