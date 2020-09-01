package api

import (
	"flag"
	"fmt"
	"log"
	"net/http"

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
}

//Server app server
var Server App

//Initialize start project
func (a *App) Initialize(dbConfig datastore.DBConfiguration, redisConfig datastore.RedisConfiguration) *App {
	a.Router = mux.NewRouter()
	//a.Router.Use(commonMiddleware)

	err := a.initializeRoutes(dbConfig, redisConfig)
	if err != nil {
		return nil
	}

	return a
}

//there are controllers that serve the http request
var questionController controller.QuestionController
var userController controller.UserController
var homeController controller.HomeController
var authController controller.AuthController
var gameController controller.GameController

//Run the project
func (a *App) Run(port string) {
	if a == nil {
		return
	}
	fmt.Println("Quiz Battle Running Now")
	log.Fatal(http.ListenAndServe(":"+port, a.Router))
}

var responseHandler = controller.GetHandler()

//initializeRoutes here we will intialize the rest apis routes and html pages
func (a *App) initializeRoutes(dbConfig datastore.DBConfiguration, redisConfig datastore.RedisConfiguration) error {
	questionRepo, errQuesRerpo := repository.NewMongoQuestionRepository(dbConfig)
	if errQuesRerpo != nil {
		return errQuesRerpo
	}
	gameRepo, errGamRerpo := repository.NewMongoGameRepository(dbConfig)
	if errGamRerpo != nil {
		return errGamRerpo
	}

	userRepo, errUserRepo := repository.NewMongoUserRepository(dbConfig)
	if errUserRepo != nil {
		return errUserRepo
	}

	//redisRerpo
	_, errRedisRerpo := repository.NewRedisCashingRepository(redisConfig)
	if errRedisRerpo != nil {
		return errRedisRerpo
	}

	questionSvc := service.NewQuestionServices(questionRepo)
	gameSvc := service.NewGameService(gameRepo, userRepo)
	createAccSvc := createaccount.NEW(userRepo)
	updateAccSvc := updateaccount.NEW(userRepo)
	loginSvc := login.New(userRepo)

	a.Router.Handle("/api/v1/question/{id:[0-9]+}", controller.TokenAuthMiddleware(http.HandlerFunc(questionController.GetQuestionByID(questionSvc)))).Methods("GET")
	a.Router.Handle("/api/v1/game/join/{id:[0-9]+}", controller.TokenAuthMiddleware(http.HandlerFunc(gameController.JoinGame(gameSvc)))).Methods("POST")
	a.Router.Handle("/api/v1/game/join/{id:[0-9]+}/{mod}", controller.TokenAuthMiddleware(http.HandlerFunc(gameController.JoinGame(gameSvc)))).Methods("POST")
	a.Router.Handle("/api/v1/game/new", controller.TokenAuthMiddleware(http.HandlerFunc(gameController.CreateGame(gameSvc)))).Methods("POST")
	a.Router.Handle("/api/v1/user/updateuser", controller.TokenAuthMiddleware(http.HandlerFunc(userController.UpdateUser(updateAccSvc)))).Methods("POST")

	a.Router.HandleFunc("/user/createuser", userController.CreateUser(createAccSvc)).Methods("POST")
	a.Router.HandleFunc("/user/login", userController.Login(loginSvc)).Methods("POST")

	a.Router.HandleFunc("/", homeController.HomePage).Methods("GET")
	a.Router.HandleFunc("/user/profile/{username}", userController.UserProfilePage).Methods("GET")

	a.Router.HandleFunc("/about", homeController.AboutPage).Methods("GET")
	a.Router.HandleFunc("/contact", homeController.ContactPage).Methods("GET")
	a.Router.HandleFunc("/auth/signin", authController.SignInPage).Methods("GET")
	a.Router.HandleFunc("/auth/signup", authController.SignUpPage).Methods("GET")
	a.Router.HandleFunc("/game/play", gameController.PlayPage).Methods("GET")
	a.Router.HandleFunc("/home/contact", homeController.ContactUS).Methods("POST")
	a.Router.HandleFunc("/battle/{id:[0-9]+}", gameController.BattlePage).Methods("GET")

	a.Router.Handle("/game/publicbattles", controller.TokenAuthMiddleware(http.HandlerFunc(gameController.GetPublicBattles(gameSvc)))).Methods("GET")
	a.Router.Handle("/game/mybattles", controller.TokenAuthMiddleware(http.HandlerFunc(gameController.GetMyBattles(gameSvc)))).Methods("GET")

	a.Router.Handle("/ws/{token}/{id:[0-9]+}", controller.TokenAuthMiddleware(http.HandlerFunc(serveGameBattle)))
	a.Router.Handle("/ws/{token}", controller.TokenAuthMiddleware(http.HandlerFunc(serveGameStream)))
	a.Router.Handle("/ws/worldchat/{token}", controller.TokenAuthMiddleware(http.HandlerFunc(serveWorldChatStream)))
	a.Router.Handle("/ws/voice/{token}", controller.TokenAuthMiddleware(http.HandlerFunc(serveVoiceChatStream)))

	var dir string

	flag.StringVar(&dir, "dir", ".", "the directory to serve files from. Defaults to the current dir")
	flag.Parse()
	// This will serve files under http://localhost:8000/static/<filename>
	a.Router.PathPrefix("/static/").Handler(http.StripPrefix("/", http.FileServer(http.Dir(dir))))
	return nil
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/home" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, "./api/view/home.html")
}

func serveGameBattle(w http.ResponseWriter, r *http.Request) {
	userData, err := controller.ExtractTokenMetadata(r)
	if err != nil {
		responseHandler.RespondWithError(w, http.StatusUnauthorized, "Can't retrive user data")
		return
	}

	websockets.ServeGameBattle(userData.UserID, userData.Fullname, w, r)
}

func serveGameStream(w http.ResponseWriter, r *http.Request) {
	userData, err := controller.ExtractTokenMetadata(r)
	if err != nil {
		responseHandler.RespondWithError(w, http.StatusUnauthorized, "Can't retrive user data")
		return
	}

	websockets.ServeGameStream(userData.UserID, userData.Fullname, w, r)
}

func serveWorldChatStream(w http.ResponseWriter, r *http.Request) {
	userData, err := controller.ExtractTokenMetadata(r)
	if err != nil {
		responseHandler.RespondWithError(w, http.StatusUnauthorized, "Can't retrive user data")
		return
	}

	websockets.ServeWorldChatStream(userData.UserID, userData.Fullname, w, r)
}

func serveVoiceChatStream(w http.ResponseWriter, r *http.Request) {
	userData, err := controller.ExtractTokenMetadata(r)
	if err != nil {
		responseHandler.RespondWithError(w, http.StatusUnauthorized, "Can't retrive user data")
		return
	}

	websockets.ServeVoiceChatStream(userData.UserID, userData.Fullname, w, r)
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
