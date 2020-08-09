package handler

import (
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

	"github.com/akorwash/QuizBattle/actor"
	"github.com/akorwash/QuizBattle/datastore"
	gameengine "github.com/akorwash/QuizBattle/gameengine"
)

//Startup to do
type Startup struct {
	Errors error
}

const (
	//NumberOFQuestion to do
	NumberOFQuestion = 100
	//MinNumOfBots to do
	MinNumOfBots = 25
	//MaxNubOfBots to do
	MaxNubOfBots = 63
	//MinBotLevel to do
	MinBotLevel = 1
	//MaxBotLevel to do
	MaxBotLevel = 25
	//MinNumOfCards to do
	MinNumOfCards = 100
	//MaxNubOfCards to do
	MaxNubOfCards = 250
	//GameFilePath to do
	GameFilePath = "\\QuizBattle\\"
)

//StartUp to do
func StartUp() *Startup {
	createBaseDirectory()
	client, err := datastore.MyDBContext.InitializingDB()
	if err != nil {
		fmt.Println(err.Error())
	}
	datastore.MyDBContext.LoadDB(client)
	rand.Seed(time.Now().UnixNano())
	return &Startup{}
}

func createBaseDirectory() {
	usr, err := user.Current()
	if err != nil {
		fmt.Println(err)
	}

	if _, err := os.Stat(usr.HomeDir + GameFilePath); os.IsNotExist(err) {
		os.Mkdir(usr.HomeDir+GameFilePath, os.ModeDir)
	}
	datastore.BaseDirectory = usr.HomeDir + GameFilePath
}

//AssignQuestionsToCards to do
func (startup *Startup) AssignQuestionsToCards() *Startup {

	if startup.Errors != nil {
		return startup
	}

	if len(gameengine.CardsSet) == 0 {
		startup.Errors = errors.New("No Cards Found")
		return startup
	}

	if len(gameengine.QuestionSet) == 0 {
		startup.Errors = errors.New("No Questions Found")
		return startup
	}

	for index, card := range gameengine.CardsSet {
		gameengine.CardsSet[index] = *card.CalculateQuestions()
	}
	datastore.MyDBContext.SaveDB()
	return startup
}

//LoadQuestions to do
func (startup *Startup) LoadQuestions() *Startup {
	if startup.Errors != nil || len(gameengine.QuestionSet) > 0 {
		return startup
	}

	for i := 1; i <= NumberOFQuestion; i++ {
		question := gameengine.NewQuestion(i, "test question #"+strconv.Itoa(i))
		gameengine.QuestionSet = append(gameengine.QuestionSet, *question)
	}
	return startup
}

//LoadBots to do
func (startup *Startup) LoadBots() *Startup {
	if startup.Errors != nil {
		return startup
	}

	numOfBots := rand.Intn(MaxNubOfBots-MinNumOfBots+1) + MinNumOfBots
	for i := 1; i <= numOfBots; i++ {
		//Calculate the name of bot
		var buffer bytes.Buffer
		buffer.WriteString("Bot #")
		buffer.WriteString(strconv.Itoa(i))

		//randomize the hardness level of the bot
		level := rand.Intn(MaxBotLevel-MinBotLevel+1) + MinBotLevel

		//create the bot account
		bot := actor.NewBot(buffer.String(), level)
		actor.BotSet = append(actor.BotSet, *bot)
	}
	return startup
}

//LoadCards to do
func (startup *Startup) LoadCards() *Startup {
	if startup.Errors != nil || len(gameengine.CardsSet) > 0 {
		return startup
	}

	numberOfQuestions := rand.Intn(MaxNubOfCards-MinNumOfCards+1) + MinNumOfCards

	for i := 1; i <= numberOfQuestions; i++ {
		//create the bot account
		card := gameengine.NewCard(i)
		gameengine.CardsSet = append(gameengine.CardsSet, *card)
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
