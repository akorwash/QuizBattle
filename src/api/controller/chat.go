package controller

import (
	"context"
	"net/http"

	"github.com/akorwash/QuizBattle/resources"
)

type ChatHistoryReader interface {
	Recent(ctx context.Context) ([]resources.ChatMessage, error)
}

type ChatController struct{}

func (controller *ChatController) Messages(chatService ChatHistoryReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := Identity(r); err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		messages, err := chatService.Recent(r.Context())
		if err != nil {
			respondServiceError(w, err)
			return
		}
		responseHandler.RespondWithJSON(w, http.StatusOK, messages)
	}
}
