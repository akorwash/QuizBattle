package engine

import (
	"QuizBattle/actor"
	"fmt"
	"time"
)

//StartNewGame to do
func StartNewGame(user *actor.User) {
	StartSessionForUser()
	time.Sleep(250 * time.Microsecond)
	fmt.Println("Thanks for choice Quiz Battle Game")
	time.Sleep(250 * time.Microsecond)
	fmt.Println("Hello: ", user.GetUserName())
}
