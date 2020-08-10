package controller

import (
	"net/http"

	"github.com/akorwash/QuizBattle/handler"
)

var responseHandler handler.WebResponseHandler

//Validate to do
func validationModel(w http.ResponseWriter, validationModel handler.IValidateInput, data string, errMess string) bool {
	if !validationModel.Validate(data) {
		responseHandler.RespondWithError(w, http.StatusBadRequest, errMess)
		return false
	}
	return true
}
