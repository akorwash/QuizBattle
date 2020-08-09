package datastore

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	gameengine "github.com/akorwash/QuizBattle/gameengine"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/akorwash/QuizBattle/actor"
	"github.com/akorwash/QuizBattle/datastore/entites"
)

//DBContext to do
type DBContext struct {
}

//MyDBContext to do
var MyDBContext DBContext

//BaseDirectory to do
var BaseDirectory string

//InitializingDB to do
func (_context *DBContext) InitializingDB() (*firestore.Client, error) {
	opt := option.WithCredentialsFile("./datastore/quizbattle-7a33e-firebase-adminsdk-tbg5l-51a241ca6a.json")

	FireBaseApp, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return nil, fmt.Errorf("error initializing app: %v", err)
	}

	client, err := FireBaseApp.Firestore(context.Background())
	if err != nil {
		return nil, fmt.Errorf("error initializing Database: %v", err)
	}
	return client, nil
}

//LoadUsers get name of Bot
func (_context *DBContext) loadUsers(client *firestore.Client) *DBContext {

	iter := client.Collection("users").Documents(context.Background())
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("Failed to iterate: %v", err)
		}
		// convert map to json
		jsonString, _ := json.Marshal(doc.Data())
		_user := entites.User{}
		json.Unmarshal(jsonString, &_user)
		user := actor.NewUser(_user.Username, _user.Password, _user.Email, _user.MobileNumber)
		actor.UserSet = append(actor.UserSet, *user)
	}
	return _context
}

//loadQuestions get name of Bot
func (_context *DBContext) loadQuestions() *DBContext {
	if _, err := os.Stat(BaseDirectory + entites.QuestionsFileName); os.IsNotExist(err) {
		ioutil.WriteFile(BaseDirectory+entites.QuestionsFileName, nil, 0644)
	}

	var questions []entites.Question
	file, _ := ioutil.ReadFile(BaseDirectory + entites.QuestionsFileName)

	_ = json.Unmarshal([]byte(file), &questions)

	for i := 0; i < len(questions); i++ {
		question := gameengine.NewQuestion(questions[i].ID, questions[i].Header)
		for _, _answer := range questions[i].Answers {
			answer := gameengine.NewAnswer(_answer.ID, _answer.Text, _answer.IsCorrect)
			question.AddAnswers(answer)
		}
		gameengine.QuestionSet = append(gameengine.QuestionSet, *question)
	}
	return _context
}

//loadCards get name of Bot
func (_context *DBContext) loadCards() *DBContext {
	if _, err := os.Stat(BaseDirectory + entites.CardFileName); os.IsNotExist(err) {
		ioutil.WriteFile(BaseDirectory+entites.CardFileName, nil, 0644)
	}

	var cards []entites.Card
	file, _ := ioutil.ReadFile(BaseDirectory + entites.CardFileName)

	_ = json.Unmarshal([]byte(file), &cards)

	for i := 0; i < len(cards); i++ {
		card := gameengine.NewLoadCard(cards[i].ID, cards[i].Power, cards[i].Owner, cards[i].Likes, cards[i].Hits)
		card.AssignQuestion(*card.GetQuestionByID(cards[i].Questions.ID))
		gameengine.CardsSet = append(gameengine.CardsSet, *card)
	}
	return _context
}

//SaveUsers to do
func (_context *DBContext) saveUsers(client *firestore.Client) *DBContext {
	for _, _user := range actor.UserSet {
		client.Collection("users").Add(context.Background(), map[string]interface{}{
			"Username":     _user.GetUserName(),
			"Password":     _user.GetPassword(),
			"Email":        _user.GetEmail(),
			"MobileNumber": _user.GetMobileNumber(),
		})
	}
	return _context
}

//SaveUsers to do
func (_context *DBContext) saveQuestions() *DBContext {
	var questions []entites.Question

	for _, _question := range gameengine.QuestionSet {
		var _answers []entites.Answer
		var answers []gameengine.Answer = *_question.GetAnswers()
		for i := 0; i < len(answers); i++ {
			_answers = append(_answers, entites.Answer{ID: answers[i].GetID(), Text: answers[i].GetText(), IsCorrect: answers[i].GetIsCorrect()})
		}
		questions = append(questions, entites.Question{ID: *_question.GetID(), Header: *_question.GetHeader(), Answers: _answers})
	}

	if !removeFile(BaseDirectory + entites.QuestionsFileName) {
		return _context
	}

	file, _ := json.MarshalIndent(questions, "", " ")
	_ = ioutil.WriteFile(BaseDirectory+entites.QuestionsFileName, file, 0644)
	return _context
}

//SaveUsers to do
func (_context *DBContext) saveCards() *DBContext {
	var cards []entites.Card

	for _, _card := range gameengine.CardsSet {
		_id, _power, _owner, _likes, _hits := _card.GetCardData()
		_question := _card.GetCardQuestion()
		var card entites.Card = entites.Card{ID: _id, Power: _power, Owner: _owner, Likes: _likes, Hits: _hits}

		var _answers []entites.Answer
		var answers []gameengine.Answer = *_question.GetAnswers()
		for i := 0; i < len(answers); i++ {
			_answers = append(_answers, entites.Answer{ID: answers[i].GetID(), Text: answers[i].GetText(), IsCorrect: answers[i].GetIsCorrect()})
		}
		card.Questions = entites.Question{ID: *_question.GetID(), Header: *_question.GetHeader(), Answers: _answers}
		cards = append(cards, card)
	}

	if !removeFile(BaseDirectory + entites.CardFileName) {
		return _context
	}

	file, _ := json.MarshalIndent(cards, "", " ")
	_ = ioutil.WriteFile(BaseDirectory+entites.CardFileName, file, 0644)
	return _context
}

//LoadDB to do
func (_context *DBContext) LoadDB(client *firestore.Client) {
	MyDBContext.loadUsers(client).loadQuestions().loadCards()
	defer client.Close()
}

//SaveDB to do
func (_context *DBContext) SaveDB() {
	client, err := MyDBContext.InitializingDB()
	if err != nil {
		fmt.Println(err.Error())
	}
	MyDBContext.saveUsers(client).saveQuestions().saveCards()
	defer client.Close()
}

func removeFile(path string) bool {
	err := os.Remove(path)
	if err != nil {
		fmt.Println(err)
		return false
	}
	return true
}
