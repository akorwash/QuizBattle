package main

import (
	"QuizBattle/datastore"
	"QuizBattle/engine"
	"QuizBattle/handler"
	"QuizBattle/service/createaccountservice"
	"QuizBattle/service/loginservice"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

func main() {
	a := App{}
	a.Initialize(
		os.Getenv("APP_DB_USERNAME"),
		os.Getenv("APP_DB_PASSWORD"),
		os.Getenv("APP_DB_NAME"))

	a.Run(":8010")
	//Loading the game
	engine.StartTheGame()

	//Intaite the Game
	gameEngine := *handler.StartUp().LoadBots().LoadCards().LoadQuestions().AssignQuestionsToCards()

	if gameEngine.Errors != nil {
		fmt.Println("unexpected error: \nerr:", gameEngine.Errors)
		return
	}

	//start recieve inputs from the user
	for {
		//display options for user
		engine.MainDialog()
		engine.Game.ReadConsoleMessage()
		var userInput string
		fmt.Scanf("%s", &userInput)

		switch userInput {
		case "1":
			fmt.Println("Welcom at Quiz Battle Game")
			fmt.Println("We Will Register Your Account Now  \n ")

			user := createaccountservice.CreateAccount(createaccountservice.RecieveUserInputs())

			engine.CardsSet.GetRandomCard().AssignToUser(user)
			engine.CardsSet.GetRandomCard().AssignToUser(user)
			engine.CardsSet.GetRandomCard().AssignToUser(user)
			datastore.MyDBContext.SaveDB()

			fmt.Println("User Info: ")
			fmt.Println(*user.GetUser())

			fmt.Println("User Cards")
			fmt.Println(*engine.GetUserCards(user.GetUserName()))
			break
		case "2":
			fmt.Println("Please Enter Your Username/Email/Mobile")
			engine.Game.ReadConsoleMessage()
			_id := engine.ReadString()

			fmt.Println("Please Enter Your Password")
			engine.Game.ReadConsoleMessage()
			_pass := engine.ReadString()

			loginModel := loginservice.LoginFactory(_id, _pass)
			switch loginservice.Login(loginModel) {
			case true:
				handler.ClearConsole()
				if engine.StartNewGame(loginModel.GetUser(_id), handler.ClearConsole) {
					handler.ClearConsole()
					break
				}
				engine.ExitTheGame()
				return
			case false:
				handler.ClearConsole()
				fmt.Println("Identifier or password wrong \n ")
				break
			}
			break
		case "4":
			engine.ExitTheGame()
			return
		case "3":
		default:
			handler.ClearConsole()
			break
		}
	}

}

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

func (a *App) getProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

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

	fmt.Println(id)
	respondWithJSON(w, http.StatusOK, id)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func (a *App) initializeRoutes() {
	a.Router.HandleFunc("/product/{id:[0-9]+}", a.getProduct).Methods("GET")
}
