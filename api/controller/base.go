package controller

import (
	"github.com/akorwash/QuizBattle/handler"
)

var responseHandler handler.WebResponseHandler

//GetHandler to do
func GetHandler() handler.WebResponseHandler {
	return responseHandler
}
