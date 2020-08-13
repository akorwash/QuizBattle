package service

import (
	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/resources"
)

//IQuestionServices services interface to create account
type IQuestionServices interface {
	GetQuestionByID(_id int) (*entites.Question, error)
}

//ILoginServices interface for login service
type ILoginServices interface {
	Login() (bool, *entites.User, error)
	GetUser(_id string) (*entites.User, error)
}

//ICreateAccountServices services interface to create account
type ICreateAccountServices interface {
	CrateUser(user entites.User) (*resources.UserAccount, error)
}
