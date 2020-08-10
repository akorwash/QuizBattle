package main

import (
	"fmt"
	"os"

	"github.com/akorwash/QuizBattle/api"
	"github.com/akorwash/QuizBattle/handler"
)

func main() {
	//Intaite the Game
	gameEngine := *handler.StartUp().LoadBots().LoadQuestions().LoadCards().AssignQuestionsToCards()
	if gameEngine.Errors != nil {
		fmt.Println("unexpected error: \nerr:", gameEngine.Errors)
		return
	}

	api.Server.Initialize().Run(os.Getenv("PORT"))
}
