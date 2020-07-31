package main

import (
	"QuizBattle/actor"
	"fmt"
)

func main() {
	user := actor.NewUser("dasdasd", "asdasd", "sadasd", "asdasd")
	bot := actor.NewBot("Bot #1", 10)
	fmt.Println(user)
	fmt.Println(bot)
}
