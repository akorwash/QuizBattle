package service

import (
	"github.com/akorwash/QuizBattle/resources"
)

// IGameServices services interface to create account
type IGameServices interface {
	CreateNewGame(userID int64, model resources.CreateGameModel) (*resources.Game, error)
	JoinGame(userID int64, gameID int64) (*resources.Game, error)
	ExitGame(userID int64, gameID int64) (*resources.Game, error)
	GetBattle(userID int64, gameID int64) (*resources.Game, error)
	CanAccessBattle(userID int64, gameID int64) error
	GetPublicBattles() ([]resources.Game, error)
	GetMyBattles(userID int64) ([]resources.Game, error)
}

// IQuestionServices services interface to create account
type IQuestionServices interface {
	GetQuestionByID(_id int) (*resources.Question, error)
}

// ICreateAccountServices services interface to create account
type ICreateAccountServices interface {
	CreateUser(user resources.CreateAccountModel) (*resources.UserAccount, error)
}

// IUpdateAccountServices services interface to create account
type IUpdateAccountServices interface {
	UpdateUser(userID int64, user resources.UpdateAccountModel) (*resources.UserAccount, error)
}
