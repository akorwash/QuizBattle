package login

import (
	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/repository"
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

//GetUser to do
func (loginModel UsernameLoginModel) GetUser(_id string) (*entites.User, error) {
	var userRepo repository.UserRepository
	user, err := userRepo.GetUserByName(loginModel.username)
	if err != nil {
		return nil, err
	}
	return user, nil
}
