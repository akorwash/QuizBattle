package handler

import (
	"encoding/json"
	"net/http"
)

//WebResponseHandler to handle any http response error/success
type WebResponseHandler struct {
}

var responseHandler WebResponseHandler

//RespondWithError return error response for user
func (a *WebResponseHandler) RespondWithError(w http.ResponseWriter, code int, message string) {
	responseHandler.RespondWithJSON(w, code, map[string]string{"error": message})
}

//RespondWithJSON return response for user maybe error or any response
func (a *WebResponseHandler) RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {

	ResponseWriter, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(ResponseWriter)
}
