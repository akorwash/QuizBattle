package handler

import (
	"QuizBattle/actor"
	"QuizBattle/datastore"
	"QuizBattle/engine"
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"time"
)

//Startup to do
type Startup struct {
	Errors error
}

//GameFilePath to do
var GameFilePath = "\\QuizBattle\\"

//StartUp to do
func StartUp() *Startup {

	usr, err := user.Current()
	if err != nil {
		fmt.Println(err)
	}

	if _, err := os.Stat(usr.HomeDir + GameFilePath); os.IsNotExist(err) {
		os.Mkdir(usr.HomeDir+GameFilePath, os.ModeDir)
	}

	datastore.BaseDirectory = usr.HomeDir + GameFilePath

	fmt.Println(datastore.BaseDirectory)
	datastore.MyDBContext.LoadDB()
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
		startup.Errors = errors.New("No Questions Found")
		return startup
	}

	for index, card := range engine.CardsSet {
		engine.CardsSet[index] = *card.CalculateQuestions()
	}
	return startup
}

//LoadQuestions to do
func (startup *Startup) LoadQuestions() *Startup {
	if startup.Errors != nil {
		return startup
	}

	for i := 1; i <= 100; i++ {
		question := engine.NewQuestion(i, "test question #"+strconv.Itoa(i))
		engine.QuestionSet = append(engine.QuestionSet, *question)
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

//ClearConsole to do
func ClearConsole() {
	value, ok := clear[runtime.GOOS] //runtime.GOOS -> linux, windows, darwin etc.
	if ok {                          //if we defined a clear func for that platform:
		value() //we execute it
	} else { //unsupported platform
		panic("Your platform is unsupported! I can't clear terminal screen :(")
	}
}

var clear map[string]func() //create a map for storing clear funcs

func init() {
	clear = make(map[string]func()) //Initialize it
	clear["linux"] = func() {
		cmd := exec.Command("clear") //Linux example, its tested
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
	clear["windows"] = func() {
		cmd := exec.Command("cmd", "/c", "cls") //Windows example, its tested
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}
