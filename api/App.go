package api

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/akorwash/QuizBattle/api/controller"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/service"
	"github.com/akorwash/QuizBattle/service/createaccount"
	"github.com/akorwash/QuizBattle/service/login"

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
	questionRepo := repository.NewMongoQuestionRepository()
	userRepo := repository.NewMongoUserRepository()

	questionSvc := service.NewQuestionServices(questionRepo)
	createAccSvc := createaccount.NEW(userRepo)
	loginSvc := login.New(userRepo)

	a.Router.HandleFunc("/question/{id:[0-9]+}", questionController.GetQuestionByID(questionSvc)).Methods("GET")
	a.Router.HandleFunc("/user/createuser", userController.CreateUser(createAccSvc)).Methods("POST")
	a.Router.HandleFunc("/user/login", userController.Login(loginSvc)).Methods("POST")
	a.Router.HandleFunc("/", homeController.HomePage).Methods("GET")
}
