package controller

import (
	"net/http"
	"strconv"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/engine"
	"github.com/akorwash/QuizBattle/handler"
	"github.com/gorilla/mux"
)

//QuestionController to do
type QuestionController struct{}

var responseHandler handler.WebResponseHandler

//GetQuestionByID to do
func (controller *QuestionController) GetQuestionByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		responseHandler.RespondWithError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	_question := engine.CardsSet.GetRandomCard().GetQuestionByID(id)
	var _answers []entites.Answer
	var answers []engine.Answer = *_question.GetAnswers()
	for i := 0; i < len(answers); i++ {
		_answers = append(_answers, entites.Answer{ID: answers[i].GetID(), Text: answers[i].GetText(), IsCorrect: answers[i].GetIsCorrect()})
	}
	question := entites.Question{ID: *_question.GetID(), Header: *_question.GetHeader(), Answers: _answers}

	//respondWithJSON(w, http.StatusOK, "payload")
	responseHandler.RespondWithJSON(w, http.StatusOK, question)
}
