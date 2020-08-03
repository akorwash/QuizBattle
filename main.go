package main

import (
	"QuizBattle/engine"
	"QuizBattle/handler"
	"QuizBattle/service/createaccountservice"
	"QuizBattle/service/loginservice"
	"fmt"
)

func main() {
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
