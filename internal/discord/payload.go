package discord

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxQuestionCharacters = 300
	maxAnswerCharacters   = 55
	minPollDurationHours  = 1
	maxPollDurationHours  = 768
	maxPollAnswers        = 10
)

var customEmojiPattern = regexp.MustCompile(`^<a?:([A-Za-z0-9_]+):([0-9]+)>$`)

// Answer contains the configured text and optional emoji for one poll option.
type Answer struct {
	Text  string
	Emoji string
}

// Payload is a Discord webhook message containing a native poll.
type Payload struct {
	Poll Poll `json:"poll"`
}

// Poll contains the question, answers, duration, and selection behavior.
type Poll struct {
	Question         PollMedia    `json:"question"`
	Answers          []PollAnswer `json:"answers"`
	Duration         int          `json:"duration"`
	AllowMultiselect bool         `json:"allow_multiselect"`
}

// PollMedia contains the text and optional emoji shown in a poll.
type PollMedia struct {
	Text  string     `json:"text"`
	Emoji *PollEmoji `json:"emoji,omitempty"`
}

// PollAnswer contains one poll option.
type PollAnswer struct {
	PollMedia PollMedia `json:"poll_media"`
}

// PollEmoji contains one Discord Unicode or custom emoji identifier.
type PollEmoji struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// BuildPayload validates and constructs a Discord poll webhook payload.
func BuildPayload(question string, answers []Answer, durationHours int, allowMultiselect bool) (Payload, error) {
	payload := Payload{
		Poll: Poll{
			Question:         PollMedia{Text: question},
			Answers:          make([]PollAnswer, 0, len(answers)),
			Duration:         durationHours,
			AllowMultiselect: allowMultiselect,
		},
	}
	for _, answer := range answers {
		media := PollMedia{Text: answer.Text}
		if answer.Emoji != "" {
			emoji, err := parseEmoji(answer.Emoji)
			if err != nil {
				return Payload{}, fmt.Errorf("invalid answer emoji: %w", err)
			}
			media.Emoji = emoji
		}
		payload.Poll.Answers = append(payload.Poll.Answers, PollAnswer{PollMedia: media})
	}
	if err := validatePayload(payload); err != nil {
		return Payload{}, err
	}
	return payload, nil
}

func validatePayload(payload Payload) error {
	if err := validateText(payload.Poll.Question.Text, "question", maxQuestionCharacters); err != nil {
		return err
	}
	if payload.Poll.Question.Emoji != nil {
		return fmt.Errorf("poll question cannot include an emoji")
	}
	if len(payload.Poll.Answers) < 1 || len(payload.Poll.Answers) > maxPollAnswers {
		return fmt.Errorf("poll must have between 1 and %d answers", maxPollAnswers)
	}
	for index, answer := range payload.Poll.Answers {
		if err := validateText(answer.PollMedia.Text, fmt.Sprintf("answer %d", index+1), maxAnswerCharacters); err != nil {
			return err
		}
		if answer.PollMedia.Emoji != nil {
			if err := validatePollEmoji(*answer.PollMedia.Emoji); err != nil {
				return fmt.Errorf("answer %d has invalid emoji: %w", index+1, err)
			}
		}
	}
	if payload.Poll.Duration < minPollDurationHours || payload.Poll.Duration > maxPollDurationHours {
		return fmt.Errorf("poll duration must be between %d and %d hours", minPollDurationHours, maxPollDurationHours)
	}
	return nil
}

func validateText(text, field string, maxCharacters int) error {
	if !utf8.ValidString(text) {
		return fmt.Errorf("%s must be valid UTF-8", field)
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("%s must not be empty", field)
	}
	if characters := utf8.RuneCountInString(text); characters > maxCharacters {
		return fmt.Errorf("%s must not exceed %d Unicode code points", field, maxCharacters)
	}
	return nil
}

func parseEmoji(value string) (*PollEmoji, error) {
	if !utf8.ValidString(value) {
		return nil, fmt.Errorf("must be valid UTF-8")
	}
	if match := customEmojiPattern.FindStringSubmatch(value); match != nil {
		return &PollEmoji{ID: match[2]}, nil
	}
	if !isUnicodeEmoji(value) {
		return nil, fmt.Errorf("must be a Unicode emoji or custom Discord emoji")
	}
	return &PollEmoji{Name: value}, nil
}

func validatePollEmoji(emoji PollEmoji) error {
	if (emoji.ID == "") == (emoji.Name == "") {
		return fmt.Errorf("exactly one of id or name must be set")
	}
	if emoji.ID != "" {
		if !isNumeric(emoji.ID) {
			return fmt.Errorf("custom emoji id must be numeric")
		}
		return nil
	}
	if !isUnicodeEmoji(emoji.Name) {
		return fmt.Errorf("name must be a Unicode emoji")
	}
	return nil
}

func isUnicodeEmoji(value string) bool {
	if value == "" || !utf8.ValidString(value) {
		return false
	}
	hasSymbol := false
	for _, character := range value {
		switch {
		case unicode.IsSymbol(character):
			hasSymbol = true
		case unicode.IsMark(character), character == '\u200d':
		default:
			return false
		}
	}
	return hasSymbol
}
