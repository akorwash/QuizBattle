package service

import (
	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/resources"
)

//IGameServices services interface to create account
type IGameServices interface {
	CreateNewGame(model resources.CreateGameModel) (*resources.Game, error)
	JoinGame(userID uint64, gameID int64) (*resources.Game, error)
}

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
	CrateUser(user resources.CreateAccountModel) (*resources.UserAccount, error)
}

//ICreateAccountServices services interface to create account
type IUpdateAccountServices interface {
	UpdateUser(user resources.UpdateAccountModel) (*resources.UserAccount, error)
}
