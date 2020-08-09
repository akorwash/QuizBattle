package datastore

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	gameengine "github.com/akorwash/QuizBattle/gameengine"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/akorwash/QuizBattle/actor"
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
	// Database Config
	clientOptions := options.Client().ApplyURI("mongodb://" + os.Getenv("MongoUsername") + ":" + os.Getenv("MongoPassword") + "@ds029979.mlab.com:29979/heroku_9gr1xz3v?retryWrites=false")
	client, err := mongo.NewClient(clientOptions)
	//Set up a context required by mongo.Connect
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)
	//Cancel context to avoid memory leak
	//defer cancel()

	// Ping our db connection
	err = client.Ping(context.Background(), readpref.Primary())
	if err != nil {
		log.Fatal("Couldn't connect to the database", err)
		return _context
	}
	log.Println("Connected!")

	// Connect to the database
	mongoContext = client.Database("heroku_9gr1xz3v")
	return _context
}

//LoadUsers get name of Bot
func (_context *DBContext) loadUsers() *DBContext {
	iter := mongoContext.Collection("users")
	cursor, err := iter.Find(context.Background(), bson.M{})
	if err != nil {
		println("Error while getting all todos, Reason: %v\n", err)
		return _context
	}
	for cursor.Next(context.Background()) {
		var _user entites.User
		cursor.Decode(&_user)
		user := actor.NewUser(_user.Username, _user.Password, _user.Email, _user.MobileNumber)
		actor.UserSet = append(actor.UserSet, *user)
	}
	return _context
}

//loadQuestions get name of Bot
func (_context *DBContext) loadQuestions() *DBContext {

	iter := mongoContext.Collection("Question")
	cursor, err := iter.Find(context.Background(), bson.M{})
	if err != nil {
		println("Error while getting all todos, Reason: %v\n", err)
		return _context
	}

	for cursor.Next(context.Background()) {
		var _question entites.Question
		cursor.Decode(&_question)
		question := gameengine.NewQuestion(_question.ID, _question.Header)
		for _, _answer := range _question.Answers {
			answer := gameengine.NewAnswer(_answer.ID, _answer.Text, _answer.IsCorrect)
			question.AddAnswers(answer)
		}
		gameengine.QuestionSet = append(gameengine.QuestionSet, *question)
	}

	return _context
}

//loadCards get name of Bot
func (_context *DBContext) loadCards() *DBContext {

	iter := mongoContext.Collection("Card")
	cursor, err := iter.Find(context.Background(), bson.M{})
	if err != nil {
		println("Error while getting all todos, Reason: %v\n", err)
		return _context
	}
	for cursor.Next(context.Background()) {
		var _card entites.Card
		cursor.Decode(&_card)
		card := gameengine.NewLoadCard(_card.ID, _card.Power, _card.Owner, _card.Likes, _card.Hits)
		card.AssignQuestion(*card.GetQuestionByID(_card.Questions.ID))
		gameengine.CardsSet = append(gameengine.CardsSet, *card)
	}

	return _context
}

//SaveUsers to do
func (_context *DBContext) saveUsers() *DBContext {
	iter := mongoContext.Collection("users")
	cursor, err := iter.Find(context.Background(), bson.M{})
	if err != nil {
		println("Error while getting all todos, Reason: %v\n", err)
		return _context
	}
	for cursor.Next(context.Background()) {
		var _user entites.User
		cursor.Decode(&_user)
		iter.DeleteOne(context.Background(), _user)
	}

	for _, _user := range actor.UserSet {

		user := entites.User{Username: _user.GetUserName(), Password: _user.GetPassword(), Email: _user.GetEmail(), MobileNumber: _user.GetMobileNumber()}
		iter.InsertOne(context.Background(), user)
	}
	return _context
}

//SaveUsers to do
func (_context *DBContext) saveQuestions() *DBContext {
	collection := mongoContext.Collection("Question")
	cursor, err := collection.Find(context.Background(), bson.M{})
	if err != nil {
		println("Error while getting all todos, Reason: %v\n", err)
		return _context
	}
	for cursor.Next(context.Background()) {
		var _question entites.Question
		cursor.Decode(&_question)
		collection.DeleteOne(context.Background(), _question)
	}

	for _, _question := range gameengine.QuestionSet {
		var answers []gameengine.Answer = *_question.GetAnswers()
		var answersMap []entites.Answer

		for i := 0; i < len(answers); i++ {
			answersMap = append(answersMap, entites.Answer{
				ID:        answers[i].GetID(),
				Text:      answers[i].GetText(),
				IsCorrect: answers[i].GetIsCorrect(),
			})
		}

		question := entites.Question{ID: *_question.GetID(), Header: *_question.GetHeader(), Answers: answersMap}
		rES, err := collection.InsertOne(context.Background(), question)

		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("Inserted multiple documents: ", rES.InsertedID)
	}

	return _context
}

//SaveUsers to do
func (_context *DBContext) saveCards() *DBContext {

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

//LoadDB to do
func (_context *DBContext) LoadDB() {
	MyDBContext.loadUsers().loadQuestions().loadCards()
}

//SaveDB to do
func (_context *DBContext) SaveDB() {
	MyDBContext.saveUsers().saveQuestions().saveCards()
}

func removeFile(path string) bool {
	err := os.Remove(path)
	if err != nil {
		fmt.Println(err)
		return false
	}
	return true
}
