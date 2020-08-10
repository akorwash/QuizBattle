package login

import (
	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/repository"
)

//UsernameLoginModel username login factory
type UsernameLoginModel struct {
	username string
	password string
}

//NewUsernameLogin create factory for username login
func NewUsernameLogin(_identifier string, _pass string) *UsernameLoginModel {
	loginModel := UsernameLoginModel{username: _identifier, password: _pass}
	return &loginModel
}

//Login login using username and password
func (loginModel UsernameLoginModel) Login() (bool, *entites.User, error) {
	var userRepo repository.UserRepository
	user, err := userRepo.GetUserByName(loginModel.username)
	if err == nil {
		if user != nil {
			return user.ValidatePassword(loginModel.password), user, nil
		}
	}
	return false, user, nil
}

//GetUser get the user information using username
func (loginModel UsernameLoginModel) GetUser(_id string) (*entites.User, error) {
	var userRepo repository.UserRepository
	user, err := userRepo.GetUserByName(loginModel.username)
	if err != nil {
		return nil, err
	}
	return user, nil
}
