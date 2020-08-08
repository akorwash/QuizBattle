package datastore

import (
	"QuizBattle/actor"
	"QuizBattle/datastore/entites"
	"QuizBattle/engine"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

//DBContext to do
type DBContext struct {
}

//MyDBContext to do
var MyDBContext DBContext

//BaseDirectory to do
var BaseDirectory string

//LoadUsers get name of Bot
func (_context *DBContext) loadUsers() *DBContext {
	if _, err := os.Stat(BaseDirectory + entites.UsersFileName); os.IsNotExist(err) {
		ioutil.WriteFile(BaseDirectory+entites.UsersFileName, nil, 0644)
	}

	var users []entites.User
	file, _ := ioutil.ReadFile(BaseDirectory + entites.UsersFileName)

	_ = json.Unmarshal([]byte(file), &users)

	for i := 0; i < len(users); i++ {
		user := actor.NewUser(users[i].Username, users[i].Password, users[i].Email, users[i].MobileNumber)
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
		question := engine.NewQuestion(questions[i].ID, questions[i].Header)
		for _, _answer := range questions[i].Answers {
			answer := engine.NewAnswer(_answer.ID, _answer.Text, _answer.IsCorrect)
			question.AddAnswers(answer)
		}
		engine.QuestionSet = append(engine.QuestionSet, *question)
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
		card := engine.NewLoadCard(cards[i].ID, cards[i].Power, cards[i].Owner, cards[i].Likes, cards[i].Hits)
		card.AssignQuestion(*card.GetQuestionByID(cards[i].Questions.ID))
		engine.CardsSet = append(engine.CardsSet, *card)
	}
	return _context
}

//SaveUsers to do
func (_context *DBContext) saveUsers() *DBContext {
	var users []entites.User
	for _, _user := range actor.UserSet {
		users = append(users, entites.User{Username: _user.GetUserName(), Password: _user.GetPassword(), Email: _user.GetEmail(), MobileNumber: _user.GetMobileNumber()})
	}

	if !removeFile(BaseDirectory + entites.UsersFileName) {
		return _context
	}

	file, _ := json.MarshalIndent(users, "", " ")
	_ = ioutil.WriteFile(BaseDirectory+entites.UsersFileName, file, 0644)
	return _context
}

//SaveUsers to do
func (_context *DBContext) saveQuestions() *DBContext {
	var questions []entites.Question

	for _, _question := range engine.QuestionSet {
		var _answers []entites.Answer
		var answers []engine.Answer = *_question.GetAnswers()
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

	for _, _card := range engine.CardsSet {
		_id, _power, _owner, _likes, _hits := _card.GetCardData()
		_question := _card.GetCardQuestion()
		var card entites.Card = entites.Card{ID: _id, Power: _power, Owner: _owner, Likes: _likes, Hits: _hits}

		var _answers []entites.Answer
		var answers []engine.Answer = *_question.GetAnswers()
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
