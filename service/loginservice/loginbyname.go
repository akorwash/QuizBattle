package loginservice

import (
	"QuizBattle/actor"
)

//UsernameLoginModel to do
type UsernameLoginModel struct {
	username string
	password string
}

//NewUsernameLogin to do
func NewUsernameLogin(_identifier string, _pass string) *UsernameLoginModel {
	loginModel := UsernameLoginModel{username: _identifier, password: _pass}
	return &loginModel
}

//Login to do
func (loginModel UsernameLoginModel) Login() bool {

	user := actor.GetUserByName(loginModel.username)
	if user != nil {
		return user.ValidatePassword(loginModel.password)
	}
	return false
}

//GetUser to do
func (loginModel UsernameLoginModel) GetUser(_id string) *actor.User {
	user := actor.GetUserByName(loginModel.username)
	return user
}
