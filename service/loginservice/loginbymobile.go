package loginservice

import (
	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/repository"
)

//MobileLoginModel to do
type MobileLoginModel struct {
	mobileNumber string
	password     string
}

//NewMobileLogin to do
func NewMobileLogin(_identifier string, _pass string) *MobileLoginModel {
	loginModel := MobileLoginModel{mobileNumber: _identifier, password: _pass}
	return &loginModel
}

//Login to do
func (loginModel MobileLoginModel) Login() (bool, *entites.User, error) {
	var userRepo repository.UserRepository
	user, err := userRepo.GetUserByMobile(loginModel.mobileNumber)
	if err == nil {
		if user != nil {
			return user.ValidatePassword(loginModel.password), user, nil
		}
	}
	return false, user, nil
}

//GetUser to do
func (loginModel MobileLoginModel) GetUser(_id string) (*entites.User, error) {
	var userRepo repository.UserRepository
	user, err := userRepo.GetUserByMobile(loginModel.mobileNumber)
	if err != nil {
		return nil, err
	}
	return user, nil
}
