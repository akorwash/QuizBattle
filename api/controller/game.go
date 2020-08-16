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

		question, err := svc.CreateNewGame(_gameModel)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, "Can't retrive question data")
			return
		}
		if question == nil {
			responseHandler.RespondWithError(w, http.StatusNotFound, "This question not found")
			return
		}
		//respondWithJSON(w, http.StatusOK, "payload")
		responseHandler.RespondWithJSON(w, http.StatusOK, *question)
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
		question, err := svc.JoinGame(gameID)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, "Can't retrive question data")
			return
		}
		if question == nil {
			responseHandler.RespondWithError(w, http.StatusNotFound, "This question not found")
			return
		}
		//respondWithJSON(w, http.StatusOK, "payload")
		responseHandler.RespondWithJSON(w, http.StatusOK, *question)
	}
}
