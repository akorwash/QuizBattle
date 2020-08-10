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
	//StartUp responsible for intialize the database and game engine, any configuration...
	gameEngine := *handler.StartUp()
	if gameEngine.Errors != nil {
		fmt.Println("unexpected error: \nerr:", gameEngine.Errors)
		return
	}

	//here we will start the game server to activate REST apis also html...
	fmt.Println("Starting Game Server")
	api.Server.Initialize().Run(os.Getenv("PORT"))
}
