package createaccountservice

import (
	"github.com/akorwash/QuizBattle/actor"
	"github.com/akorwash/QuizBattle/datastore"
)

//CreateAccount to do
func CreateAccount(_username, _pass, _email, _mobNum string) *actor.User {
	user := actor.NewUser(_username, _pass, _email, _mobNum)
	actor.UserSet = append(actor.UserSet, *user)
	datastore.MyDBContext.SaveDB()
	return user
}
