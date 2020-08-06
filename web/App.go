package web

import (
	"QuizBattle/datastore/entites"
	"QuizBattle/engine"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

//App web
type App struct {
	Router *mux.Router
	DB     *sql.DB
}

//Initialize start project
func (a *App) Initialize(user, password, dbname string) {
	//connectionString :=
	//fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable", user, password, dbname)

	//var err error
	//a.DB, err = sql.Open("postgres", connectionString)
	//if err != nil {
	//	log.Fatal(err)
	//}

	a.Router = mux.NewRouter()

	a.initializeRoutes()
}

//Run the project
func (a *App) Run(addr string) {
	log.Fatal(http.ListenAndServe(":8010", a.Router))
}

func (a *App) initializeRoutes() {
	a.Router.HandleFunc("/question/{id:[0-9]+}", a.getQuestionByID).Methods("GET")
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSONPyload(w http.ResponseWriter, code int, payload []byte) {

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(payload)
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {

	ResponseWriter, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(ResponseWriter)
}

func (a *App) getQuestionByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid product ID")
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
	            respondWithError(w, http.StatusNotFound, "Product not found")
	        default:
	            respondWithError(w, http.StatusInternalServerError, err.Error())
	        }
	        return
		}*/

	fmt.Println(question)
	//respondWithJSON(w, http.StatusOK, "payload")
	respondWithJSON(w, http.StatusOK, question)
}
