package handler

import (
	"math/rand"
	"time"

	"github.com/akorwash/QuizBattle/datastore"
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
)

//StartUp responsible for intiate the database, randmoize with UNIX Nano
func StartUp() *Startup {
	//Intialize the database, to prepare the context and run the seed mthods
	datastore.MyDBContext.InitializingDB()
	//somtimes we need to generate randome numbers, we use this to seed the randome numbers
	rand.Seed(time.Now().UnixNano())
	return &Startup{}
}
