package controller

import (
	"net/http"

	"github.com/akorwash/QuizBattle/resources"
	"github.com/akorwash/QuizBattle/service"
)

type MatchController struct{}

func (controller *MatchController) Prepare(matchService *service.MatchService) http.HandlerFunc {
	return controller.command(matchService, "prepare")
}

func (controller *MatchController) CommitDeck(matchService *service.MatchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := Identity(r)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		gameID, err := pathID(r, "id")
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, "invalid battle ID")
			return
		}
		var input resources.CommitDeckModel
		if err := decodeJSON(w, r, &input); err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		cardIDs, err := service.ParsePublicIDs(input.CardIDs)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, "invalid card IDs")
			return
		}
		snapshot, err := matchService.CommitDeck(r.Context(), identity.UserID, gameID, cardIDs, input.CommandID)
		if err != nil {
			respondServiceError(w, err)
			return
		}
		responseHandler.RespondWithJSON(w, http.StatusOK, snapshot)
	}
}

func (controller *MatchController) Start(matchService *service.MatchService) http.HandlerFunc {
	return controller.command(matchService, "start")
}

func (controller *MatchController) Forfeit(matchService *service.MatchService) http.HandlerFunc {
	return controller.command(matchService, "forfeit")
}

func (controller *MatchController) Snapshot(matchService *service.MatchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := Identity(r)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		gameID, err := pathID(r, "id")
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, "invalid battle ID")
			return
		}
		snapshot, err := matchService.Snapshot(r.Context(), identity.UserID, gameID)
		if err != nil {
			respondServiceError(w, err)
			return
		}
		responseHandler.RespondWithJSON(w, http.StatusOK, snapshot)
	}
}

func (controller *MatchController) Answer(matchService *service.MatchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := Identity(r)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		gameID, err := pathID(r, "id")
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, "invalid battle ID")
			return
		}
		var input resources.SubmitAnswerModel
		if err := decodeJSON(w, r, &input); err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		snapshot, err := matchService.Answer(r.Context(), identity.UserID, gameID, input.TurnID, input.Option, input.CommandID)
		if err != nil {
			respondServiceError(w, err)
			return
		}
		responseHandler.RespondWithJSON(w, http.StatusOK, snapshot)
	}
}

func (controller *MatchController) command(matchService *service.MatchService, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := Identity(r)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		gameID, err := pathID(r, "id")
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, "invalid battle ID")
			return
		}
		var input resources.CommandModel
		if err := decodeJSON(w, r, &input); err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		if action != "prepare" && action != "start" && action != "forfeit" {
			responseHandler.RespondWithError(w, http.StatusBadRequest, "invalid match command")
			return
		}
		var snapshot any
		if action == "prepare" {
			snapshot, err = matchService.Prepare(r.Context(), identity.UserID, gameID, input.CommandID)
		} else if action == "start" {
			snapshot, err = matchService.Start(r.Context(), identity.UserID, gameID, input.CommandID)
		} else {
			snapshot, err = matchService.Forfeit(r.Context(), identity.UserID, gameID, input.CommandID)
		}
		if err != nil {
			respondServiceError(w, err)
			return
		}
		responseHandler.RespondWithJSON(w, http.StatusOK, snapshot)
	}
}
