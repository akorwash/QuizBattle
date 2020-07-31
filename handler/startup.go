package handler

import (
	"QuizBattle/actor"
	"QuizBattle/engine"
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

//Startup to do
type Startup struct {
	Errors error
}

//StartUp to do
func StartUp() *Startup {
	rand.Seed(time.Now().UnixNano())
	return &Startup{}
}

//AssignQuestionsToCards to do
func (startup *Startup) AssignQuestionsToCards() *Startup {
	if startup.Errors != nil {
		return startup
	}

	if len(engine.CardsSet) <= 0 {
		startup.Errors = errors.New("No Cards Found")
		return startup
	}

	if len(engine.QuestionSet) <= 0 {
		fmt.Println("No Questions Found")
		startup.Errors = errors.New("No Questions Found")
		return startup
	}

	for _, card := range engine.CardsSet {
		card.CalculateQuestions()
	}
	return startup
}

//LoadQuestions to do
func (startup *Startup) LoadQuestions() *Startup {
	if startup.Errors != nil {
		return startup
	}

	return startup
}

//LoadBots to do
func (startup *Startup) LoadBots() *Startup {
	if startup.Errors != nil {
		return startup
	}
	min := 25
	max := 63

	numOfBots := rand.Intn(max-min+1) + min
	for i := 1; i <= numOfBots; i++ {
		//Calculate the name of bot
		var buffer bytes.Buffer
		buffer.WriteString("Bot #")
		buffer.WriteString(strconv.Itoa(i))

		//randomize the hardness level of the bot
		min := 1
		max := 25
		level := rand.Intn(max-min+1) + min

		//create the bot account
		bot := actor.NewBot(buffer.String(), level)
		actor.BotSet = append(actor.BotSet, *bot)
	}
	return startup
}

//LoadCards to do
func (startup *Startup) LoadCards() *Startup {
	if startup.Errors != nil {
		return startup
	}

	min := 100
	max := 250

	numberOfQuestions := rand.Intn(max-min+1) + min
	for i := 1; i <= numberOfQuestions; i++ {
		//create the bot account
		card := engine.NewCard(i)
		engine.CardsSet = append(engine.CardsSet, *card)
	}
	return startup
}
