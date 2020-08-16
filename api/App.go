package api

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"

	"github.com/akorwash/QuizBattle/api/controller"
	"github.com/akorwash/QuizBattle/datastore"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/service"
	"github.com/akorwash/QuizBattle/service/createaccount"
	"github.com/akorwash/QuizBattle/service/login"
	"github.com/akorwash/QuizBattle/service/updateaccount"
	"github.com/akorwash/QuizBattle/websockets"

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
func (a *App) Initialize(dbConfig datastore.DBConfiguration) *App {
	a.Router = mux.NewRouter()
	err := a.initializeRoutes(dbConfig)
	if err != nil {
		return nil
	}
	return a
}

//there are controllers that serve the http request
var questionController controller.QuestionController
var userController controller.UserController
var homeController controller.HomeController
var gameController controller.GameController

//Run the project
func (a *App) Run(port string) {
	if a == nil {
		return
	}
	log.Fatal(http.ListenAndServe(":"+port, a.Router))
}

var responseHandler = controller.GetHandler()

//initializeRoutes here we will intialize the rest apis routes and html pages
func (a *App) initializeRoutes(dbConfig datastore.DBConfiguration) error {
	questionRepo, errQuesRerpo := repository.NewMongoQuestionRepository(dbConfig)
	if errQuesRerpo != nil {
		println("Error while get database context For Repo: %v\n", errQuesRerpo)
		return errQuesRerpo
	}
	gameRepo, errGamRerpo := repository.NewMongoGameRepository(dbConfig)
	if errGamRerpo != nil {
		println("Error while get database context For Repo: %v\n", errGamRerpo)
		return errGamRerpo
	}
	userRepo, errUserRepo := repository.NewMongoUserRepository(dbConfig)
	if errUserRepo != nil {
		println("Error while get database context For Repo: %v\n", errUserRepo)
		return errUserRepo
	}
	questionSvc := service.NewQuestionServices(questionRepo)
	gameSvc := service.NewGameService(gameRepo)
	createAccSvc := createaccount.NEW(userRepo)
	updateAccSvc := updateaccount.NEW(userRepo)
	loginSvc := login.New(userRepo)

	a.Router.HandleFunc("/question/{id:[0-9]+}", questionController.GetQuestionByID(questionSvc)).Methods("GET")
	a.Router.HandleFunc("/game/join/{id:[0-9]+}", gameController.JoinGame(gameSvc)).Methods("POST")
	a.Router.HandleFunc("/game/new", gameController.CreateGame(gameSvc)).Methods("POST")
	a.Router.HandleFunc("/user/createuser", userController.CreateUser(createAccSvc)).Methods("POST")
	a.Router.HandleFunc("/user/updateuser", userController.UpdateUser(updateAccSvc)).Methods("POST")
	a.Router.HandleFunc("/user/login", userController.Login(loginSvc)).Methods("POST")
	a.Router.HandleFunc("/", homeController.HomePage).Methods("GET")
	a.Router.HandleFunc("/home", serveHome)
	a.Router.HandleFunc("/ws/{id:[0-9]+}", serveWS)
	return nil
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.URL.Path != "/home" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, "./api/home.html")
}

func serveWS(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		responseHandler.RespondWithError(w, http.StatusBadRequest, "Invalid question ID")
		return
	}
	websockets.ServeWs(id, w, r)
}
