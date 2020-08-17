package controller

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/akorwash/QuizBattle/resources"
	"github.com/akorwash/QuizBattle/service"
	"github.com/gorilla/mux"
)

//GameController game controller
type GameController struct{} //GetQuestionByID  handle get question by id http request

//CreateGame create new game battle
func (controller *GameController) CreateGame(svc service.IGameServices) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var _gameModel resources.CreateGameModel
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&_gameModel); err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}
		defer r.Body.Close()

		userData, err := ExtractTokenMetadata(r)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "Can't retrive user data")
			return
		}

		if userData.UserID != _gameModel.UserID {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "you don't have access to create game for another user")
			return
		}

		game, err := svc.CreateNewGame(_gameModel)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, "Can't create game due err: "+err.Error())
			return
		}

		if game == nil {
			responseHandler.RespondWithError(w, http.StatusNotFound, "Game not created try again later")
			return
		}
		//respondWithJSON(w, http.StatusOK, "payload")
		responseHandler.RespondWithJSON(w, http.StatusOK, *game)
	}
}

//JoinGame create new game battle
func (controller *GameController) JoinGame(svc service.IGameServices) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		gameID, err := strconv.Atoi(vars["id"])
		if err != nil {
			gameID = 0
		}

		userData, err := ExtractTokenMetadata(r)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "Can't retrive user data")
			return
		}
		game, err := svc.JoinGame(userData.UserID, int64(gameID))

		if err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, "Can't create game due err: "+err.Error())
			return
		}

		if game == nil {
			responseHandler.RespondWithError(w, http.StatusNotFound, "Game not created try again later")
			return
		}

		//respondWithJSON(w, http.StatusOK, "payload")
		responseHandler.RespondWithJSON(w, http.StatusOK, *game)
	}
}
