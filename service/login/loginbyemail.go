package login

import (
	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/repository"
)

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
func (loginModel EmailLoginModel) Login() (bool, *entites.User, error) {
	var userRepo repository.UserRepository
	user, err := userRepo.GetUserByEmail(loginModel.email)
	if err == nil {
		if user != nil {
			return user.ValidatePassword(loginModel.password), user, nil
		}
	}
	return false, user, nil
}

//GetUser to do
func (loginModel EmailLoginModel) GetUser(_id string) (*entites.User, error) {
	var userRepo repository.UserRepository
	user, err := userRepo.GetUserByEmail(loginModel.email)
	if err != nil {
		return nil, err
	}
	return user, nil
}
