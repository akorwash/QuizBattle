package api

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/akorwash/QuizBattle/api/controller"

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
func (a *App) Initialize() {
	a.Router = mux.NewRouter()
	a.initializeRoutes()
}

var questionController controller.QuestionController
var userController controller.UserController

//Run the project
func (a *App) Run(port string) {
	log.Fatal(http.ListenAndServe(":"+port, a.Router))
}

func (a *App) initializeRoutes() {
	a.Router.HandleFunc("/question/{id:[0-9]+}", questionController.GetQuestionByID).Methods("GET")
	a.Router.HandleFunc("/user/createuser", userController.CreateUser).Methods("POST")
}
