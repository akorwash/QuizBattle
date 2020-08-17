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
	a.Router.Use(commonMiddleware)

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

	a.Router.Handle("/api/v1/question/{id:[0-9]+}", controller.TokenAuthMiddleware(http.HandlerFunc(questionController.GetQuestionByID(questionSvc)))).Methods("GET")
	a.Router.Handle("/api/v1/game/join/{id:[0-9]+}", controller.TokenAuthMiddleware(http.HandlerFunc(gameController.JoinGame(gameSvc)))).Methods("POST")
	a.Router.Handle("/api/v1/game/new", controller.TokenAuthMiddleware(http.HandlerFunc(gameController.CreateGame(gameSvc)))).Methods("POST")
	a.Router.HandleFunc("/user/createuser", userController.CreateUser(createAccSvc)).Methods("POST")
	a.Router.Handle("/api/v1/user/updateuser", controller.TokenAuthMiddleware(http.HandlerFunc(userController.UpdateUser(updateAccSvc)))).Methods("POST")
	a.Router.HandleFunc("/user/login", userController.Login(loginSvc)).Methods("POST")
	a.Router.HandleFunc("/", homeController.HomePage).Methods("GET")
	a.Router.HandleFunc("/home", serveHome)
	a.Router.Handle("/api/v1/ws/{id:[0-9]+}", controller.TokenAuthMiddleware(http.HandlerFunc(serveWS)))
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

func commonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Access-Control-Request-Headers, Access-Control-Request-Method, Connection, Host, Origin, User-Agent, Referer, Cache-Control, X-header")
		next.ServeHTTP(w, r)
	})
}
