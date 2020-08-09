package main

import (
	"fmt"
	"os"

	"github.com/akorwash/QuizBattle/api"
	gameengine "github.com/akorwash/QuizBattle/gameengine"
	"github.com/akorwash/QuizBattle/handler"
	"github.com/akorwash/QuizBattle/service/loginservice"
	_ "github.com/lib/pq"
)

func main() {
	//Intaite the Game
	gameEngine := *handler.StartUp().LoadBots().LoadQuestions().LoadCards().AssignQuestionsToCards()
	api.Server.Initialize()

	api.Server.Run(os.Getenv("PORT"))

	if gameEngine.Errors != nil {
		fmt.Println("unexpected error: \nerr:", gameEngine.Errors)
		return
	}

	//start recieve inputs from the user
	for {
		//display options for user
		gameengine.MainDialog()
		gameengine.Game.ReadConsoleMessage()
		var userInput string
		fmt.Scanf("%s", &userInput)

		switch userInput {
		case "2":
			fmt.Println("Please Enter Your Username/Email/Mobile")
			gameengine.Game.ReadConsoleMessage()
			_id := gameengine.ReadString()

			fmt.Println("Please Enter Your Password")
			gameengine.Game.ReadConsoleMessage()
			_pass := gameengine.ReadString()

			loginModel := loginservice.LoginFactory(_id, _pass)
			switch loginservice.Login(loginModel) {
			case true:
				handler.ClearConsole()
				if gameengine.StartNewGame(loginModel.GetUser(_id), handler.ClearConsole) {
					handler.ClearConsole()
					break
				}
				gameengine.ExitTheGame()
				return
			case false:
				handler.ClearConsole()
				fmt.Println("Identifier or password wrong \n ")
				break
			}
			break
		case "4":
			gameengine.ExitTheGame()
			return
		case "3":
		default:
			handler.ClearConsole()
			break
		}
	}

}
