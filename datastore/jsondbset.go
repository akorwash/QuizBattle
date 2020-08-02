package datastore

import (
	"QuizBattle/actor"
	"QuizBattle/datastore/entites"
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
func (_context *DBContext) loadUsers() {
	if _, err := os.Stat(BaseDirectory + "users.json"); os.IsNotExist(err) {
		ioutil.WriteFile(BaseDirectory+"users.json", nil, 0644)
	}

	var users []entites.UserEntity
	file, _ := ioutil.ReadFile(BaseDirectory + "users.json")

	_ = json.Unmarshal([]byte(file), &users)

	for i := 0; i < len(users); i++ {
		user := actor.NewUser(users[i].Username, users[i].Password, users[i].Email, users[i].MobileNumber)
		actor.UserSet = append(actor.UserSet, *user)
	}
}

//SaveUsers to do
func (_context *DBContext) saveUsers() {
	var users []entites.UserEntity
	for _, _user := range actor.UserSet {
		users = append(users, entites.UserEntity{Username: _user.GetUserName(), Password: _user.GetPassword(), Email: _user.GetEmail(), MobileNumber: _user.GetMobileNumber()})
	}

	file, _ := json.MarshalIndent(users, "", " ")
	_ = ioutil.WriteFile(BaseDirectory+"users.json", file, 0644)
}

//LoadDB to do
func (_context *DBContext) LoadDB() {
	MyDBContext.loadUsers()
}

//SaveDB to do
func (_context *DBContext) SaveDB() {
	MyDBContext.saveUsers()
}
