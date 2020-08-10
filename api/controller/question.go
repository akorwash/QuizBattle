package controller

import (
	"net/http"
	"strconv"

	"github.com/akorwash/QuizBattle/repository"
	"github.com/gorilla/mux"
)

//QuestionController to do
type QuestionController struct{}

//GetQuestionByID to do
func (controller *QuestionController) GetQuestionByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		responseHandler.RespondWithError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	var questionRepo repository.QuestionRepository
	question, err := questionRepo.GetQuestionByID(id)
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
