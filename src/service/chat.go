package service

import (
	"context"
	"fmt"

	chatdomain "github.com/akorwash/QuizBattle/domain/chat"
	"github.com/akorwash/QuizBattle/resources"
)

const recentChatMessageLimit int64 = 50

// ChatRepository is the persistence boundary owned by the chat service.
type ChatRepository interface {
	Save(ctx context.Context, message *chatdomain.Message) error
	ListRecent(ctx context.Context, limit int64) ([]chatdomain.Message, error)
}

// ChatServices is the narrow application interface consumed by HTTP and
// WebSocket transports.
type ChatServices interface {
	Save(ctx context.Context, userID int64, username, fullName, text string) (*resources.ChatMessage, error)
	Recent(ctx context.Context) ([]resources.ChatMessage, error)
}

type ChatService struct {
	repository ChatRepository
}

func NewChatService(repository ChatRepository) *ChatService {
	return &ChatService{repository: repository}
}

// Save accepts only server-authored identity fields. A WebSocket handler must
// pass values from its authenticated connection, never values from JSON.
func (service *ChatService) Save(ctx context.Context, userID int64, username, fullName, text string) (*resources.ChatMessage, error) {
	message, err := chatdomain.NewMessage(userID, username, fullName, text)
	if err != nil {
		return nil, fmt.Errorf("%w: message must be plain text between 1 and %d characters", ErrInvalidInput, chatdomain.MaxTextRunes)
	}
	if err := service.repository.Save(ctx, message); err != nil {
		return nil, err
	}
	result := mapChatMessage(*message)
	return &result, nil
}

func (service *ChatService) Recent(ctx context.Context) ([]resources.ChatMessage, error) {
	messages, err := service.repository.ListRecent(ctx, recentChatMessageLimit)
	if err != nil {
		return nil, err
	}
	result := make([]resources.ChatMessage, 0, len(messages))
	for _, message := range messages {
		result = append(result, mapChatMessage(message))
	}
	return result, nil
}

func mapChatMessage(message chatdomain.Message) resources.ChatMessage {
	return resources.ChatMessage{
		ID:        message.ID,
		UserID:    message.UserID,
		Username:  message.Username,
		FullName:  message.FullName,
		Message:   message.Text,
		CreatedAt: message.CreatedAt,
	}
}
