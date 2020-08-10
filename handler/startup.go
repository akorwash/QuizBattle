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

//StartUp to do
func StartUp() *Startup {
	datastore.MyDBContext.InitializingDB()
	rand.Seed(time.Now().UnixNano())
	return &Startup{}
}
