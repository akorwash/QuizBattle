package api

import (
	"QuizBattle/api/controller"
	"database/sql"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

//App web
type App struct {
	Router *mux.Router
	DB     *sql.DB
}

//Server to do
var Server App

//Initialize start project
func (a *App) Initialize(user, password, dbname string) {
	a.Router = mux.NewRouter()
	a.initializeRoutes()
}

var questionController controller.QuestionController

//Run the project
func (a *App) Run(addr string) {
	log.Fatal(http.ListenAndServe(":8010", a.Router))
}

func (a *App) initializeRoutes() {
	a.Router.HandleFunc("/question/{id:[0-9]+}", questionController.GetQuestionByID).Methods("GET")
}
