package actor

import . "github.com/ahmetb/go-linq"

//User is class represent Player User account with login data
type User struct {
	username     string
	password     string
	email        string
	mobileNumber string
}

//UserModel is View model of User Account
type UserModel struct {
	username     string
	email        string
	mobileNumber string
}

//UserList to do
type UserList []User

//UserSet to do
var UserSet UserList

//NewUser ctor for User Account
func NewUser(name string, pass string, _email string, mobNum string) *User {
	user := User{username: name, password: pass, email: _email, mobileNumber: mobNum}
	return &user
}

//GetUserByName to do
func GetUserByName(_name string) *User {
	var owners []User

	From(UserSet).Where(func(c interface{}) bool {
		return c.(User).username == _name
	}).Select(func(c interface{}) interface{} {
		return c.(User)
	}).ToSlice(&owners)

	if len(owners) == 1 {
		return &owners[0]
	} else {
		return nil
	}
}

//GetUserByMobile to do
func GetUserByMobile(_mobile string) *User {
	var owners []User

	From(UserSet).Where(func(c interface{}) bool {
		return c.(User).mobileNumber == _mobile
	}).Select(func(c interface{}) interface{} {
		return c.(User)
	}).ToSlice(&owners)

	if len(owners) == 1 {
		return &owners[0]
	} else {
		return nil
	}
}

//GetUserByEmail to do
func GetUserByEmail(_email string) *User {
	var owners []User

	From(UserSet).Where(func(c interface{}) bool {
		return c.(User).email == _email
	}).Select(func(c interface{}) interface{} {
		return c.(User)
	}).ToSlice(&owners)

	if len(owners) == 1 {
		return &owners[0]
	} else {
		return nil
	}
}

//ValidatePassword get name of Bot
func (userAccount *User) ValidatePassword(_pass string) bool {
	return (userAccount.password == _pass)
}

//GetUserName get name of Bot
func (userAccount *User) GetUserName() string {
	return userAccount.username
}

//GetUserNamePointer get name of Bot
func (userAccount *User) GetUserNamePointer() *string {
	return &userAccount.username
}

//GetEmail get name of Bot
func (userAccount *User) GetEmail() string {
	return userAccount.email
}

//GetMobileNumber get name of Bot
func (userAccount *User) GetMobileNumber() string {
	return userAccount.mobileNumber
}

//GetUser get name of Bot
func (userAccount *User) GetUser() *UserModel {
	usermodel := UserModel{username: userAccount.username, email: userAccount.email, mobileNumber: userAccount.mobileNumber}
	return &usermodel
}
