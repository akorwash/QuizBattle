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

//Server app server
var Server App

//Initialize start project
func (a *App) Initialize() *App {
	a.Router = mux.NewRouter()
	a.initializeRoutes()
	return a
}

//there are controllers that serve the http request
var questionController controller.QuestionController
var userController controller.UserController
var homeController controller.HomeController

//Run the project
func (a *App) Run(port string) {
	log.Fatal(http.ListenAndServe(":"+port, a.Router))
}

//initializeRoutes here we will intialize the rest apis routes and html pages
func (a *App) initializeRoutes() {
	a.Router.HandleFunc("/question/{id:[0-9]+}", questionController.GetQuestionByID).Methods("GET")
	a.Router.HandleFunc("/user/createuser", userController.CreateUser).Methods("POST")
	a.Router.HandleFunc("/user/login", userController.Login).Methods("POST")
	a.Router.HandleFunc("/", homeController.HomePage).Methods("GET")
}
