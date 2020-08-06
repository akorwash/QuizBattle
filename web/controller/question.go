package controller

import (
	"QuizBattle/datastore/entites"
	"QuizBattle/engine"
	"QuizBattle/handler"
	"fmt"
	"net/http"
	"strconv"

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
	var _answers []entites.AnswerEntity
	var answers []engine.Answer = *_question.GetAnswers()
	for i := 0; i < len(answers); i++ {
		_answers = append(_answers, entites.AnswerEntity{ID: answers[i].GetID(), Text: answers[i].GetText(), IsCorrect: answers[i].GetIsCorrect()})
	}
	question := entites.QuestionEntity{ID: *_question.GetID(), Header: *_question.GetHeader(), Answers: _answers}
	/*p := product{ID: id}
	    if err := p.getProduct(a.DB); err != nil {
	        switch err {
	        case sql.ErrNoRows:
	            responseHandler.respondWithError(w, http.StatusNotFound, "Product not found")
	        default:
	            responseHandler.respondWithError(w, http.StatusInternalServerError, err.Error())
	        }
	        return
		}*/

	fmt.Println(question)
	//respondWithJSON(w, http.StatusOK, "payload")
	responseHandler.RespondWithJSON(w, http.StatusOK, question)
}
