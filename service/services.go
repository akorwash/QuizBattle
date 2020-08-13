package service

import "github.com/akorwash/QuizBattle/datastore/entites"

//IQuestionServices services interface to create account
type IQuestionServices interface {
	GetQuestionByID(_id int) (*entites.Question, error)
}

//ILoginServices interface for login service
type ILoginServices interface {
	Login() (bool, *entites.User, error)
	GetUser(_id string) (*entites.User, error)
}
