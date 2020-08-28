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

	dbConfig := handler.GetDBConfig()
	redisConfig := handler.GetRedisConfig()
	gameEngine := *handler.StartUp(dbConfig)
	if gameEngine.Errors != nil {
		fmt.Println("unexpected error: \nerr:", gameEngine.Errors)
		return
	}

	//here we will start the game server to activate REST apis also html...
	fmt.Println("Starting Game Server")
	api.Server.Initialize(dbConfig, redisConfig).Run(os.Getenv("PORT"))
}
