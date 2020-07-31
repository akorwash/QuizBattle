package actor

//User is class represent Player User account with login data
type user struct {
	username     string
	password     string
	email        string
	mobileNumber string
}

//NewUser ctor for User Account
//user is User Account
func NewUser(name string, pass string, _email string, mobNum string) *user {
	user := user{username: name, password: pass, email: _email, mobileNumber: mobNum}
	return &user
}
