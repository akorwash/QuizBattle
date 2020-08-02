package loginservice

import "QuizBattle/actor"

//EmailLoginModel to do
type EmailLoginModel struct {
	email    string
	password string
}

//NewEmailLogin to do
func NewEmailLogin(_identifier string, _pass string) *EmailLoginModel {
	loginModel := EmailLoginModel{email: _identifier, password: _pass}
	return &loginModel
}

//Login to do
func (loginModel EmailLoginModel) Login() bool {
	user := actor.GetUserByEmail(loginModel.email)
	if user != nil {
		return user.ValidatePassword(loginModel.password)
	}
	return false
}
