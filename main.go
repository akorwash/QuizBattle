package main

import (
	"fmt"
	"os"

	"github.com/akorwash/QuizBattle/api"
	"github.com/akorwash/QuizBattle/handler"
)

func main() {
	fmt.Println("Starting Game Engine")
	//Intaite the Game
	gameEngine := *handler.StartUp()
	if gameEngine.Errors != nil {
		fmt.Println("unexpected error: \nerr:", gameEngine.Errors)
		return
	}
	fmt.Println("Starting Game Server")
	api.Server.Initialize().Run(os.Getenv("PORT"))
}
