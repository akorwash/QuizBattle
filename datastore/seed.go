package datastore

import (
	"bytes"
	"context"
	"math/rand"
	"os"
	"strconv"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"go.mongodb.org/mongo-driver/bson"
)

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

//SeedInitializer to do
type SeedInitializer struct {
}

//Seed to do
func (seed *SeedInitializer) Seed() {
	seedUsers()
	seedBots()
	seedQuestions()
}

func seedUsers() {
	dbcontext, cancelContext, err := GetContext()
	if err != nil {
		println("Error while get database context: %v\n", err)
		defer cancelContext()
		return
	}
	iter := dbcontext.Collection("users")
	cursor, err := iter.CountDocuments(context.Background(), bson.M{})
	if err != nil {
		println("Error while count users recored: %v\n", err)
		defer cancelContext()
		return
	}

	if cursor <= 0 {
		//create the bot account
		user := entites.User{Username: os.Getenv("AdminUserAccount"), Email: "admin@hosta.care", MobileNumber: os.Getenv("AdminUserMobile"), Password: os.Getenv("AdminUserPassword")}
		iter.InsertOne(context.Background(), user)
	}
	defer cancelContext()
}

func seedBots() {
	dbcontext, cancelContext, err := GetContext()
	if err != nil {
		println("Error while get database context: %v\n", err)
		defer cancelContext()
		return
	}
	iter := dbcontext.Collection("bots")
	cursor, err := iter.CountDocuments(context.Background(), bson.M{})
	if err != nil {
		println("Error while count bots recored: %v\n", err)
		defer cancelContext()
		return
	}

	if cursor <= 0 {
		numOfBots := rand.Intn(MaxNubOfBots-MinNumOfBots+1) + MinNumOfBots
		for i := 1; i <= numOfBots; i++ {
			//Calculate the name of bot
			var buffer bytes.Buffer
			buffer.WriteString("Bot #")
			buffer.WriteString(strconv.Itoa(i))

			//randomize the hardness level of the bot
			level := rand.Intn(MaxBotLevel-MinBotLevel+1) + MinBotLevel

			//create the bot account
			bot := entites.Bot{ID: i, BotName: buffer.String(), Level: level}
			iter.InsertOne(context.Background(), bot)
		}
	}
	defer cancelContext()
}

func seedQuestions() {
	dbcontext, cancelContext, err := GetContext()
	if err != nil {
		println("Error while get database context: %v\n", err)
		defer cancelContext()
		return
	}
	iter := dbcontext.Collection("Question")
	cursor, err := iter.CountDocuments(context.Background(), bson.M{})
	if err != nil {
		println("Error while count questions recored: %v\n", err)
		defer cancelContext()
		return
	}

	if cursor <= 0 {
		var answersForQuestion1 []entites.Answer
		answersForQuestion1 = append(answersForQuestion1, entites.Answer{ID: 1, Text: "تونس", IsCorrect: false})
		answersForQuestion1 = append(answersForQuestion1, entites.Answer{ID: 2, Text: "مصر", IsCorrect: true})
		answersForQuestion1 = append(answersForQuestion1, entites.Answer{ID: 3, Text: "السعودية", IsCorrect: false})
		answersForQuestion1 = append(answersForQuestion1, entites.Answer{ID: 4, Text: "العراق", IsCorrect: false})
		question1 := entites.Question{ID: 1, Header: "اين صنعت أول كسوة للكعبة؟", Answers: answersForQuestion1}
		iter.InsertOne(context.Background(), question1)

		var answersForQuestion2 []entites.Answer
		answersForQuestion2 = append(answersForQuestion2, entites.Answer{ID: 1, Text: "الطائف", IsCorrect: true})
		answersForQuestion2 = append(answersForQuestion2, entites.Answer{ID: 2, Text: "الدمام", IsCorrect: false})
		answersForQuestion2 = append(answersForQuestion2, entites.Answer{ID: 3, Text: "الخبر", IsCorrect: false})
		answersForQuestion2 = append(answersForQuestion2, entites.Answer{ID: 4, Text: "الرياض", IsCorrect: false})
		question2 := entites.Question{ID: 2, Header: "في اي مدينة يتواجد سوق عكاظ؟", Answers: answersForQuestion2}
		iter.InsertOne(context.Background(), question2)

		var answersForQuestion4 []entites.Answer
		answersForQuestion4 = append(answersForQuestion4, entites.Answer{ID: 1, Text: "جسر السلطان سليم الأول", IsCorrect: false})
		answersForQuestion4 = append(answersForQuestion4, entites.Answer{ID: 2, Text: "جسر هايوان كوينغداو", IsCorrect: false})
		answersForQuestion4 = append(answersForQuestion4, entites.Answer{ID: 3, Text: "جسر دانيانغ-كونشان", IsCorrect: false})
		answersForQuestion4 = append(answersForQuestion4, entites.Answer{ID: 4, Text: "جسر الملك فهد", IsCorrect: true})
		question4 := entites.Question{ID: 4, Header: "ماهو اطول جسر بحري في العالم؟", Answers: answersForQuestion4}
		iter.InsertOne(context.Background(), question4)
	}
	defer cancelContext()
}
