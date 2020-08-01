package main

import (
	"QuizBattle/actor"
	"QuizBattle/engine"
	"QuizBattle/handler"
	"fmt"
	"time"
)

func main() {

	//Loading the game
	fmt.Print("Loading ")
	for i := 0; i < 10; i++ {
		time.Sleep(250 * time.Millisecond)
		fmt.Print("-")
	}
	fmt.Println("-")

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
		engine.ReadConsoleMessage()
		var userInput string
		fmt.Scanf("%s", &userInput)

		switch userInput {
		case "1":
			fmt.Println("Thanks to choice our game")

			fmt.Println("Please Enter Your mobile number")
			engine.ReadConsoleMessage()

			_mobNum := engine.ReadString()

			fmt.Println("Please Enter Your Password")
			engine.ReadConsoleMessage()

			_pass := engine.ReadString()

			fmt.Println("Please Enter Your Usernane")
			engine.ReadConsoleMessage()

			_username := engine.ReadString()

			fmt.Println("Please Enter Your Email")
			engine.ReadConsoleMessage()

			_email := engine.ReadString()

			user := actor.NewUser(_username, _pass, _email, _mobNum)
			actor.UserSet = append(actor.UserSet, *user)

			engine.CardsSet.GetRandomCard().AssignToUser(user)
			engine.CardsSet.GetRandomCard().AssignToUser(user)
			engine.CardsSet.GetRandomCard().AssignToUser(user)

			fmt.Println("User Info: ")
			fmt.Println(*user.GetUser())

			fmt.Println("User Cards")
			fmt.Println(*engine.GetUserCards(user.GetUserName()))
			break
		case "2":
			fmt.Println("Please Enter Your mobile number")
			engine.ReadConsoleMessage()
			_mobNum := engine.ReadString()

			fmt.Println("Please Enter Your Password")
			engine.ReadConsoleMessage()
			_pass := engine.ReadString()

			fmt.Println(_mobNum, " - ", _pass)
			break
		case "3":
		default:
			handler.ClearConsole()
			break
		}
	}

}
