package controller

import (
	"net/http"

	engine "github.com/akorwash/QuizBattle/gameengine"
	"github.com/akorwash/QuizBattle/handler"
)

var responseHandler handler.WebResponseHandler

//Validate to do
func validationModel(w http.ResponseWriter, validationModel engine.IValidateInput, data string, errMess string) bool {
	if !validationModel.Validate(data) {
		responseHandler.RespondWithError(w, http.StatusBadRequest, errMess)
		return false
	}
	return true
}
