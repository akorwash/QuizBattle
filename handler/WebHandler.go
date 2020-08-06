package handler

import (
	"encoding/json"
	"net/http"
)

//WebResponseHandler to do
type WebResponseHandler struct {
}

var responseHandler WebResponseHandler

//RespondWithError to do
func (a *WebResponseHandler) RespondWithError(w http.ResponseWriter, code int, message string) {
	responseHandler.RespondWithJSON(w, code, map[string]string{"error": message})
}

//RespondWithJSONPyload to do
func (a *WebResponseHandler) RespondWithJSONPyload(w http.ResponseWriter, code int, payload []byte) {

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(payload)
}

//RespondWithJSON to do
func (a *WebResponseHandler) RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {

	ResponseWriter, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(ResponseWriter)
}
