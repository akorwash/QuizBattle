package actor

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
