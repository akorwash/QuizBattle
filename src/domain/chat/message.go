package chat

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	// MaxTextRunes is shared by every chat ingestion path. Counting runes keeps
	// the limit predictable for Arabic and other multi-byte text.
	MaxTextRunes = 500
	maxNameRunes = 80
	maxUserRunes = 32
)

var (
	ErrInvalidMessage = errors.New("chat: invalid message")
	htmlMarkup        = regexp.MustCompile(`(?is)<(?:/?[a-z][^>]*|!--.*?--|![a-z][^>]*)>`)
)

// Message is the persisted, server-authored world-chat entity. User identity
// fields come from the authenticated session, never from a client payload.
type Message struct {
	ID        int64     `bson:"id"`
	UserID    int64     `bson:"userId"`
	Username  string    `bson:"username"`
	FullName  string    `bson:"fullName"`
	Text      string    `bson:"text"`
	CreatedAt time.Time `bson:"createdAt"`
}

// NewMessage normalizes and validates chat data before it reaches storage.
// Markup is rejected instead of sanitized so the database contains the exact
// plain text that clients render with textContent.
func NewMessage(userID int64, username, fullName, text string) (*Message, error) {
	username = strings.TrimSpace(username)
	fullName = strings.TrimSpace(fullName)
	text = strings.TrimSpace(text)
	if userID <= 0 || !validIdentityText(username, maxUserRunes) || !validIdentityText(fullName, maxNameRunes) || !validText(text) {
		return nil, ErrInvalidMessage
	}
	return &Message{UserID: userID, Username: username, FullName: fullName, Text: text}, nil
}

// Validate protects repository callers that do not pass through ChatService.
// A persisted message must already be normalized; accepting a second variant
// here would make real-time and history payloads disagree.
func (message Message) Validate() error {
	normalized, err := NewMessage(message.UserID, message.Username, message.FullName, message.Text)
	if err != nil || normalized.Username != message.Username || normalized.FullName != message.FullName || normalized.Text != message.Text {
		return ErrInvalidMessage
	}
	return nil
}

func validIdentityText(value string, maximum int) bool {
	count := utf8.RuneCountInString(value)
	if !utf8.ValidString(value) || count == 0 || count > maximum || htmlMarkup.MatchString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validText(value string) bool {
	count := utf8.RuneCountInString(value)
	if !utf8.ValidString(value) || count == 0 || count > MaxTextRunes || htmlMarkup.MatchString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) && character != '\n' && character != '\t' {
			return false
		}
	}
	return true
}
