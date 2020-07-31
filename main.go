package main

import (
	"QuizBattle/actor"
	"bytes"
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

func main() {
	var bots []actor.BotPlayer

	user := actor.NewUser("Ahmed Korwash", "123456789", "email@site.com", "01024873097")
	fmt.Println(*user.GetUser())

	//randomize for number of bots
	rand.Seed(time.Now().UnixNano())
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
		bots = append(bots, *bot)
	}

	fmt.Println("Number of Bots: ", len(bots))
	for _, bot := range bots {
		fmt.Println("Bot Name: ", bot.GetName(), " Level: ", bot.GetLevel())
	}
}
