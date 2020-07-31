package main

import (
	"QuizBattle/actor"
	"QuizBattle/engine"
	"QuizBattle/handler"
	"fmt"
)

func main() {
	user := actor.NewUser("Ahmed Korwash", "123456789", "email@site.com", "01024873097")
	actor.UserSet = append(actor.UserSet, *user)

	//Intaite the Game
	gameEngine := *handler.StartUp().LoadBots().LoadCards().LoadQuestions().AssignQuestionsToCards()

	if gameEngine.Errors != nil {
		fmt.Println("unexpected error: \nerr:", gameEngine.Errors)
		return
	}

	//randomize for number of bots
	fmt.Println("Number of Bots: ", len(actor.BotSet))
	for _, bot := range actor.BotSet {
		fmt.Println("Bot Name: ", bot.GetName(), " Level: ", bot.GetLevel())
	}

	engine.CardsSet.GetRandomCard().AssignToUser(user)
	engine.CardsSet.GetRandomCard().AssignToUser(user)
	engine.CardsSet.GetRandomCard().AssignToUser(user)

	fmt.Println("User Info: ")
	fmt.Println(*user.GetUser())

	fmt.Println("User Cards")
	fmt.Println(*engine.GetUserCards(user.GetUserName()))
}
