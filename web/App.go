package web

import (
	"QuizBattle/web/controller"
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

var productController controller.QuestionController

//Run the project
func (a *App) Run(addr string) {
	log.Fatal(http.ListenAndServe(":8010", a.Router))
}

func (a *App) initializeRoutes() {
	a.Router.HandleFunc("/question/{id:[0-9]+}", productController.GetQuestionByID).Methods("GET")
}
