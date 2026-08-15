package controller

import (
	"net/http"
	"strconv"

	"github.com/akorwash/QuizBattle/resources"
	"github.com/akorwash/QuizBattle/service"
)

type GameController struct{}

func (controller *GameController) CreateGame(gameService service.IGameServices) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := Identity(r)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		var input resources.CreateGameModel
		if err := decodeJSON(w, r, &input); err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		game, err := gameService.CreateNewGame(identity.UserID, input)
		if err != nil {
			respondServiceError(w, err)
			return
		}
		responseHandler.RespondWithJSON(w, http.StatusCreated, game)
	}
}

func (controller *GameController) JoinGame(gameService service.IGameServices) http.HandlerFunc {
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
		game, err := gameService.JoinGame(identity.UserID, gameID)
		if err != nil {
			respondServiceError(w, err)
			return
		}
		responseHandler.RespondWithJSON(w, http.StatusOK, game)
	}
}

func (controller *GameController) ExitGame(gameService service.IGameServices) http.HandlerFunc {
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
		game, err := gameService.ExitGame(identity.UserID, gameID)
		if err != nil {
			respondServiceError(w, err)
			return
		}
		responseHandler.RespondWithJSON(w, http.StatusOK, game)
	}
}

func (controller *GameController) GetBattle(gameService service.IGameServices) http.HandlerFunc {
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
		game, err := gameService.GetBattle(identity.UserID, gameID)
		if err != nil {
			respondServiceError(w, err)
			return
		}
		responseHandler.RespondWithJSON(w, http.StatusOK, game)
	}
}

func (controller *GameController) GetPublicBattles(gameService service.IGameServices) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		games, err := gameService.GetPublicBattles()
		if err != nil {
			respondServiceError(w, err)
			return
		}
		responseHandler.RespondWithJSON(w, http.StatusOK, games)
	}
}

func (controller *GameController) GetMyBattles(gameService service.IGameServices) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := Identity(r)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		games, err := gameService.GetMyBattles(identity.UserID)
		if err != nil {
			respondServiceError(w, err)
			return
		}
		responseHandler.RespondWithJSON(w, http.StatusOK, games)
	}
}

func (controller *GameController) PlayPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./api/view/gameplay.html")
}

func (controller *GameController) BattlePage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./api/view/battle.html")
}

func pathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, strconv.ErrSyntax
	}
	return id, nil
}
