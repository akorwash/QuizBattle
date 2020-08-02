package createaccountservice

import (
	"QuizBattle/actor"
	"QuizBattle/datastore"
	"QuizBattle/engine"
	"QuizBattle/handler"
	"fmt"
)

//RecieveUserInputs to do
func RecieveUserInputs() (_username, _pass, _email, _mobNum string) {

	fmt.Println("Please Enter Your Usernane")
	fmt.Println("Can't start with numbers, or have a whitespace")
	fmt.Println("Password should be >= 5 char.")
	engine.ReadConsoleMessage()

	_username = engine.ReadStringWithValidation(handler.ValidateUsername{})

	fmt.Println("Please Enter Your mobile number")
	engine.ReadConsoleMessage()

	_mobNum = engine.ReadStringWithValidation(handler.ValidateMobile{})

	fmt.Println("Please Enter Your Email")
	engine.ReadConsoleMessage()

	_email = engine.ReadStringWithValidation(handler.ValidateEmail{})

	fmt.Println("Please Enter Your Password")
	fmt.Println("at least one (upper and lower) case letter.")
	fmt.Println("at least one (digit and special) character.")
	fmt.Println("Password should be >= 8 char.")
	engine.ReadConsoleMessage()

	_pass = engine.ReadStringWithValidation(handler.ValidatePassword{})

	return _username, _pass, _email, _mobNum
}

//CreateAccount to do
func CreateAccount(_username, _pass, _email, _mobNum string) *actor.User {
	user := actor.NewUser(_username, _pass, _email, _mobNum)
	actor.UserSet = append(actor.UserSet, *user)
	datastore.MyDBContext.SaveDB()
	return user
}
