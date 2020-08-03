package engine

import (
	"QuizBattle/actor"
	"fmt"
	"time"
)

//GameEngine to do
type GameEngine struct {
}

//Game to do
var Game GameEngine = GameEngine{}

//StartNewGame to do
func StartNewGame(user *actor.User, consoleClearHandler func()) bool {
	Game.StartSessionForUser()
	time.Sleep(250 * time.Microsecond)
	fmt.Println("Thanks for choice Quiz Battle Game")
	time.Sleep(250 * time.Microsecond)
	fmt.Println("Account Name: ", user.GetUserName())

	for {
		Game.MainGameDialog()
		Game.ReadConsoleMessage()
		var userInput string
		fmt.Scanf("%s", &userInput)

		switch userInput {
		case "1":
			break
		case "2":
			break
		case "3":
			break
		case "4":
			break
		case "5":
			return true
		case "6":
			consoleClearHandler()
			return false
		case "7":
		default:
			consoleClearHandler()
			break
		}
	}
}
