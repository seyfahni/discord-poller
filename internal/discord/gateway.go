// Package discord adapts DiscordGo to the application gateway interface.
package discord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/seyfahni/discord-poller/internal/app"
)

var _ app.Gateway = (*Gateway)(nil)

// Gateway manages the DiscordGo gateway session.
type Gateway struct {
	session *discordgo.Session
}

// New creates a Discord gateway with the intents required for bot operation.
func New(token string) (*Gateway, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("create Discord session: %w", err)
	}
	session.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentGuildMessagePolls |
		discordgo.IntentMessageContent
	return &Gateway{session: session}, nil
}

// Open connects the session to Discord.
func (g *Gateway) Open() error {
	if err := g.session.Open(); err != nil {
		return fmt.Errorf("open Discord session: %w", err)
	}
	return nil
}

// Close disconnects the session from Discord.
func (g *Gateway) Close() error {
	if err := g.session.Close(); err != nil {
		return fmt.Errorf("close Discord session: %w", err)
	}
	return nil
}
