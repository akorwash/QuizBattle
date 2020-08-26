package login

import (
	"github.com/akorwash/QuizBattle/datastore/entites"
)

//UsernameLoginModel username login factory
type UsernameLoginModel struct {
	username string
	password string
	services *LoginService
}

//NewUsernameLogin create factory for username login
func NewUsernameLogin(_services *LoginService, _identifier string, _pass string) *UsernameLoginModel {
	loginModel := UsernameLoginModel{services: _services, username: _identifier, password: _pass}
	return &loginModel
}

//Login login using username and password
func (loginModel UsernameLoginModel) Login() (bool, *entites.User, error) {
	user, err := loginModel.services.userRepo.GetUserByName(loginModel.username)
	if err == nil {
		if user != nil {
			return user.ValidatePassword(loginModel.password), user, nil
		}
	}
	return false, user, nil
}

//GetUser get the user information using username
func (loginModel UsernameLoginModel) GetUser(_id string) (*entites.User, error) {
	user, err := loginModel.services.userRepo.GetUserByName(loginModel.username)
	if err != nil {
		return nil, err
	}
	return user, nil
}
