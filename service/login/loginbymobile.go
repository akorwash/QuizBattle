package login

import (
	"github.com/akorwash/QuizBattle/datastore/entites"
)

//MobileLoginModel mobile login factory
type MobileLoginModel struct {
	mobileNumber string
	password     string
	services     *LoginService
}

//NewMobileLogin create factory for mobile login
func NewMobileLogin(_services *LoginService, _identifier string, _pass string) *MobileLoginModel {
	loginModel := MobileLoginModel{services: _services, mobileNumber: _identifier, password: _pass}
	return &loginModel
}

//Login login using mobile number and password
func (loginModel MobileLoginModel) Login() (bool, *entites.User, error) {
	user, err := loginModel.services.userRepo.GetUserByMobile(loginModel.mobileNumber)
	if err == nil {
		if user != nil {
			return user.ValidatePassword(loginModel.password), user, nil
		}
	}
	return false, user, nil
}

//GetUser get the user information using mobile number
func (loginModel MobileLoginModel) GetUser(_id string) (*entites.User, error) {
	user, err := loginModel.services.userRepo.GetUserByMobile(loginModel.mobileNumber)
	if err != nil {
		return nil, err
	}
	return user, nil
}
