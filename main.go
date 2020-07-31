package main

import (
	"QuizBattle/actor"
	"QuizBattle/engine"
	"QuizBattle/handler"
	"fmt"
	"math/rand"
)

func main() {
	user := actor.NewUser("Ahmed Korwash", "123456789", "email@site.com", "01024873097")
	actor.UserSet = append(actor.UserSet, *user)

	//Intaite the Game
	handler.StartUp().IntaiteBots().IntaiteCards().IntaiteQuestions()

	//randomize for number of bots
	fmt.Println("Number of Bots: ", len(actor.BotSet))
	for _, bot := range actor.BotSet {
		fmt.Println("Bot Name: ", bot.GetName(), " Level: ", bot.GetLevel())
	}

	min := 1
	max := len(engine.CardsSet)

	engine.CardsSet[rand.Intn(max-min+1)+min].AssignToUser(user)
	engine.CardsSet[rand.Intn(max-min+1)+min].AssignToUser(user)
	engine.CardsSet[rand.Intn(max-min+1)+min].AssignToUser(user)

	fmt.Println("User Info: ")
	fmt.Println(*user.GetUser())

	fmt.Println("User Cards")
	fmt.Println(*engine.GetUserCards(user.GetUserName()))
}
