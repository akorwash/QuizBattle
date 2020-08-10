package datastore

import (
	"context"
	"fmt"
	"log"

	gameengine "github.com/akorwash/QuizBattle/gameengine"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/akorwash/QuizBattle/datastore/entites"
)

//DBContext to do
type DBContext struct {
}

//MyDBContext to do
var MyDBContext DBContext

var mongoContext *mongo.Database

//BaseDirectory to do
var BaseDirectory string

//InitializingDB to do
func (_context *DBContext) InitializingDB() *DBContext {
	seedInit := SeedInitializer{}
	seedInit.Seed()
	return _context
}

//SaveCards to do
func (_context *DBContext) SaveCards() *DBContext {

	iter := mongoContext.Collection("Card")
	cursor, err := iter.Find(context.Background(), bson.M{})
	if err != nil {
		println("Error while getting all todos, Reason: %v\n", err)
		return _context
	}
	for cursor.Next(context.Background()) {
		var _card entites.Question
		cursor.Decode(&_card)
		iter.DeleteOne(context.Background(), _card)
	}

	for _, _card := range gameengine.CardsSet {
		_id, _power, _owner, _likes, _hits := _card.GetCardData()
		_question := _card.GetCardQuestion()
		var answers []gameengine.Answer = *_question.GetAnswers()
		var answersMap []entites.Answer

		for i := 0; i < len(answers); i++ {
			answersMap = append(answersMap, entites.Answer{
				ID:        answers[i].GetID(),
				Text:      answers[i].GetText(),
				IsCorrect: answers[i].GetIsCorrect(),
			})
		}
		card := entites.Card{
			ID:        _id,
			Power:     _power,
			Owner:     _owner,
			Likes:     _likes,
			Hits:      _hits,
			Questions: entites.Question{ID: *_question.GetID(), Header: *_question.GetHeader(), Answers: answersMap},
		}

		rES, err := iter.InsertOne(context.Background(), card)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("Inserted multiple documents: ", rES.InsertedID)
	}

	return _context
}

//AddUser to do
func (_context *DBContext) AddUser(user entites.User) error {
	dbcontext, cancelContext, err := GetContext()
	if err != nil {
		println("Error while get database context: %v\n", err)
		defer cancelContext()
		return err
	}
	iter := dbcontext.Collection("users")
	//create the bot account
	iter.InsertOne(context.Background(), user)
	defer cancelContext()
	return nil
}
