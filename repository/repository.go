package repository

import "github.com/akorwash/QuizBattle/datastore/entites"

//IUserRepository repository interface for users
type IUserRepository interface {
	GetUserByName(_name string) (*entites.User, error)
	GetUserByMobile(_mobile string) (*entites.User, error)
	GetUserByEmail(_email string) (*entites.User, error)
	GetUserByID(_id int64) (*entites.User, error)
	AddUser(user entites.User) error
	UpdateUser(user entites.User) error
}

//IQuestionRepository repo interface for question
type IQuestionRepository interface {
	GetQuestionByID(_id int) (*entites.Question, error)
}

//IGameRepository repo interface for question
type IGameRepository interface {
	Count() (int64, error)
	Add(game entites.Game) error
	GetGameByID(_id int64) (*entites.Game, error)
	GetPublicBattle() ([]entites.Game, error)
	GetMyBattle(usreID uint64) ([]entites.Game, error)
	JoinedGame(gameID int64, usreID []uint64) error
}
