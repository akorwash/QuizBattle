package chat

import (
	"errors"
	"strings"
	"testing"
)

func TestNewMessageNormalizesPlainText(t *testing.T) {
	message, err := NewMessage(42, " player_1 ", " لاعب أول ", "  أهلًا بالعالم  ")
	if err != nil {
		t.Fatal(err)
	}
	if message.UserID != 42 || message.Username != "player_1" || message.FullName != "لاعب أول" || message.Text != "أهلًا بالعالم" {
		t.Fatalf("message was not normalized: %+v", message)
	}
}

func TestNewMessageRejectsMarkupAndOversizedText(t *testing.T) {
	tests := []string{
		"<script>alert(1)</script>",
		"hello <strong>world</strong>",
		strings.Repeat("س", MaxTextRunes+1),
		"hello\u0000world",
	}
	for _, text := range tests {
		if _, err := NewMessage(42, "player_1", "لاعب أول", text); !errors.Is(err, ErrInvalidMessage) {
			t.Fatalf("invalid text %q returned %v", text, err)
		}
	}
	if _, err := NewMessage(42, "player_1", "لاعب أول", "2 < 3"); err != nil {
		t.Fatalf("plain mathematical text was rejected: %v", err)
	}
}

func TestNewMessageCountsUnicodeRunes(t *testing.T) {
	if _, err := NewMessage(42, "player_1", "لاعب أول", strings.Repeat("ع", MaxTextRunes)); err != nil {
		t.Fatalf("exact rune limit was rejected: %v", err)
	}
}
