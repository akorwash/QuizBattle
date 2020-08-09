package datastore

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

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
	var accServices = AccServices{
		AccountType:   os.Getenv("AccountType"),
		ProjectID:     os.Getenv("ProjectID"),
		PrivateKeyID:  os.Getenv("PrivateKeyID"),
		PrivateKey:    os.Getenv("PrivateKey"),
		ClientEmail:   os.Getenv("ClientEmail"),
		ClientID:      os.Getenv("ClientID"),
		AuthURI:       os.Getenv("AuthURI"),
		TokenURI:      os.Getenv("TokenURI"),
		AuthCERTURL:   os.Getenv("AuthCERTURL"),
		ClientCERTURL: os.Getenv("ClientCERTURL"),
	}
	accServices.PrivateKey = strings.Replace(accServices.PrivateKey, "\\n", "\n", -1)
	accServicesfile, _ := json.MarshalIndent(accServices, "", " ")
	_ = ioutil.WriteFile("./datastore/accServices.json", accServicesfile, 0644)

	opt := option.WithCredentialsFile("./datastore/accServices.json")

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
func (_context *DBContext) loadQuestions(client *firestore.Client) *DBContext {

	iter := client.Collection("Question").Documents(context.Background())
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
		_question := entites.Question{}
		json.Unmarshal(jsonString, &_question)
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
func (_context *DBContext) loadCards(client *firestore.Client) *DBContext {

	iter := client.Collection("Card").Documents(context.Background())
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
		_card := entites.Card{}
		json.Unmarshal(jsonString, &_card)
		card := gameengine.NewLoadCard(_card.ID, _card.Power, _card.Owner, _card.Likes, _card.Hits)
		card.AssignQuestion(*card.GetQuestionByID(_card.Questions.ID))
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
func (_context *DBContext) saveQuestions(client *firestore.Client) *DBContext {

	for _, _question := range gameengine.QuestionSet {
		var answers []gameengine.Answer = *_question.GetAnswers()
		var answersMap []map[string]interface{}

		for i := 0; i < len(answers); i++ {
			answersMap = append(answersMap, map[string]interface{}{
				"ID":        answers[i].GetID(),
				"Text":      answers[i].GetText(),
				"IsCorrect": answers[i].GetIsCorrect(),
			})
		}

		client.Collection("Question").Add(context.Background(), map[string]interface{}{
			"ID":      *_question.GetID(),
			"Header":  *_question.GetHeader(),
			"Answers": answersMap,
		})
	}

	return _context
}

//SaveUsers to do
func (_context *DBContext) saveCards(client *firestore.Client) *DBContext {

	for _, _card := range gameengine.CardsSet {
		_id, _power, _owner, _likes, _hits := _card.GetCardData()
		_question := _card.GetCardQuestion()
		var answers []gameengine.Answer = *_question.GetAnswers()
		var answersMap []map[string]interface{}

		for i := 0; i < len(answers); i++ {
			answersMap = append(answersMap, map[string]interface{}{
				"ID":        answers[i].GetID(),
				"Text":      answers[i].GetText(),
				"IsCorrect": answers[i].GetIsCorrect(),
			})
		}

		client.Collection("Card").Add(context.Background(), map[string]interface{}{
			"ID":    _id,
			"Power": _power,
			"Owner": _owner,
			"Likes": _likes,
			"Hits":  _hits,
			"Questions": map[string]interface{}{
				"ID":      *_question.GetID(),
				"Header":  *_question.GetHeader(),
				"Answers": answersMap,
			},
		})
	}

	return _context
}

//LoadDB to do
func (_context *DBContext) LoadDB(client *firestore.Client) {
	MyDBContext.loadUsers(client).loadQuestions(client).loadCards(client)
	defer client.Close()
}

//SaveDB to do
func (_context *DBContext) SaveDB() {
	client, err := MyDBContext.InitializingDB()
	if err != nil {
		fmt.Println(err.Error())
	}
	MyDBContext.saveUsers(client).saveQuestions(client).saveCards(client)
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
