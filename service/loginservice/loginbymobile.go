package loginservice

import (
	"QuizBattle/actor"
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
func (loginModel MobileLoginModel) Login() bool {
	user := actor.GetUserByMobile(loginModel.mobileNumber)
	if user != nil {
		return user.ValidatePassword(loginModel.password)
	}
	return false
}
