package datastore

import (
	"QuizBattle/actor"
	"QuizBattle/datastore/entites"
	"QuizBattle/engine"
	"encoding/json"
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

	var users []entites.UserEntity
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

	var questions []entites.QuestionEntity
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

//SaveUsers to do
func (_context *DBContext) saveUsers() *DBContext {
	var users []entites.UserEntity
	for _, _user := range actor.UserSet {
		users = append(users, entites.UserEntity{Username: _user.GetUserName(), Password: _user.GetPassword(), Email: _user.GetEmail(), MobileNumber: _user.GetMobileNumber()})
	}

	file, _ := json.MarshalIndent(users, "", " ")
	_ = ioutil.WriteFile(BaseDirectory+entites.UsersFileName, file, 0644)
	return _context
}

//LoadDB to do
func (_context *DBContext) LoadDB() {
	MyDBContext.loadUsers().loadQuestions()
}

//SaveDB to do
func (_context *DBContext) SaveDB() {
	MyDBContext.saveUsers()
}
