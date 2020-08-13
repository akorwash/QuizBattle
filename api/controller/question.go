package controller

import (
	"net/http"
	"strconv"

	"github.com/akorwash/QuizBattle/service"
	"github.com/gorilla/mux"
)

//QuestionController question controller
type QuestionController struct{}

//GetQuestionByID  handle get question by id http request
func (controller *QuestionController) GetQuestionByID(svc service.IQuestionServices) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, "Invalid question ID")
			return
		}

		question, err := svc.GetQuestionByID(id)
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
