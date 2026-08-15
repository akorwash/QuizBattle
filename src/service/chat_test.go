package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	chatdomain "github.com/akorwash/QuizBattle/domain/chat"
)

type chatRepositoryStub struct {
	saved     *chatdomain.Message
	messages  []chatdomain.Message
	saveErr   error
	listErr   error
	listLimit int64
}

func (stub *chatRepositoryStub) Save(_ context.Context, message *chatdomain.Message) error {
	if stub.saveErr != nil {
		return stub.saveErr
	}
	copy := *message
	copy.ID = 901
	copy.CreatedAt = time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)
	*message = copy
	stub.saved = &copy
	return nil
}

func (stub *chatRepositoryStub) ListRecent(_ context.Context, limit int64) ([]chatdomain.Message, error) {
	stub.listLimit = limit
	return stub.messages, stub.listErr
}

func TestChatServiceSavesAuthenticatedPlainText(t *testing.T) {
	repository := &chatRepositoryStub{}
	chatService := NewChatService(repository)
	message, err := chatService.Save(context.Background(), 42, "player_1", "لاعب أول", "  مرحبًا  ")
	if err != nil {
		t.Fatal(err)
	}
	if repository.saved == nil || repository.saved.UserID != 42 || repository.saved.Username != "player_1" || repository.saved.Text != "مرحبًا" {
		t.Fatalf("wrong entity persisted: %+v", repository.saved)
	}
	if message.ID != 901 || message.UserID != 42 || message.Message != "مرحبًا" || message.CreatedAt.IsZero() {
		t.Fatalf("wrong resource returned: %+v", message)
	}
}

func TestChatServiceRejectsInvalidTextBeforeStorage(t *testing.T) {
	repository := &chatRepositoryStub{}
	chatService := NewChatService(repository)
	for _, text := range []string{"<img src=x onerror=alert(1)>", strings.Repeat("س", chatdomain.MaxTextRunes+1)} {
		if _, err := chatService.Save(context.Background(), 42, "player_1", "لاعب أول", text); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid text returned %v", err)
		}
	}
	if repository.saved != nil {
		t.Fatal("invalid message reached repository")
	}
}

func TestChatServiceReadsLatestFifty(t *testing.T) {
	repository := &chatRepositoryStub{messages: []chatdomain.Message{
		{ID: 1, UserID: 41, Username: "player_1", FullName: "لاعب أول", Text: "أولًا", CreatedAt: time.Unix(1, 0).UTC()},
		{ID: 2, UserID: 42, Username: "player_2", FullName: "لاعب ثان", Text: "ثانيًا", CreatedAt: time.Unix(2, 0).UTC()},
	}}
	result, err := NewChatService(repository).Recent(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if repository.listLimit != recentChatMessageLimit || len(result) != 2 || result[0].Message != "أولًا" || result[1].Username != "player_2" {
		t.Fatalf("unexpected history: limit=%d messages=%+v", repository.listLimit, result)
	}
}

func TestChatServicePropagatesRepositoryFailure(t *testing.T) {
	want := errors.New("database unavailable")
	repository := &chatRepositoryStub{saveErr: want, listErr: want}
	chatService := NewChatService(repository)
	if _, err := chatService.Save(context.Background(), 42, "player_1", "لاعب أول", "مرحبًا"); !errors.Is(err, want) {
		t.Fatalf("save error was hidden: %v", err)
	}
	if _, err := chatService.Recent(context.Background()); !errors.Is(err, want) {
		t.Fatalf("list error was hidden: %v", err)
	}
}
