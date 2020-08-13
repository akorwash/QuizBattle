package login

import (
	"github.com/akorwash/QuizBattle/datastore/entites"
)

//EmailLoginModel email login factory
type EmailLoginModel struct {
	email    string
	password string
	services *LoginService
}

//NewEmailLogin create factory for email login
func NewEmailLogin(_services *LoginService, _identifier string, _pass string) *EmailLoginModel {
	loginModel := EmailLoginModel{services: _services, email: _identifier, password: _pass}
	return &loginModel
}

//Login login using email and password
func (loginModel EmailLoginModel) Login() (bool, *entites.User, error) {
	user, err := loginModel.services.userRepo.GetUserByEmail(loginModel.email)
	if err == nil {
		if user != nil {
			return user.ValidatePassword(loginModel.password), user, nil
		}
	}
	return false, user, nil
}

//GetUser get the user using the user email
func (loginModel EmailLoginModel) GetUser(_id string) (*entites.User, error) {
	user, err := loginModel.services.userRepo.GetUserByEmail(loginModel.email)
	if err != nil {
		return nil, err
	}
	return user, nil
}
