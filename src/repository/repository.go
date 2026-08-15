package repository

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/akorwash/QuizBattle/datastore/entites"
)

var (
	ErrNotFound = errors.New("repository: not found")
	ErrConflict = errors.New("repository: conflict")
)

// IUserRepository repository interface for users
type IUserRepository interface {
	GetUserByName(_name string) (*entites.User, error)
	GetUserByMobile(_mobile string) (*entites.User, error)
	GetUserByEmail(_email string) (*entites.User, error)
	GetUserByID(_id int64) (*entites.User, error)
	GetUsersByIDs(ids []int64) (map[int64]entites.User, error)
	AddUser(user *entites.User) error
	UpdateUser(user entites.User) error
}

// IQuestionRepository repo interface for question
type IQuestionRepository interface {
	GetQuestionByID(_id int) (*entites.Question, error)
}

// IGameRepository repo interface for question
type IGameRepository interface {
	CountActiveGame(userID int64) (int64, error)
	Add(game *entites.Game) error
	GetGameByID(_id int64) (*entites.Game, error)
	GetPublicBattle() ([]entites.Game, error)
	GetMyBattle(userID int64) ([]entites.Game, error)
	JoinGame(gameID int64, userID int64) error
	LeaveGame(gameID int64, userID int64) error
	CloseGame(gameID int64) error
}

// ICashRepository repo interface for cashing client
type ICashRepository interface {
	SetString(ctx context.Context, key string, value string, expiration time.Duration) error
	SetByte(ctx context.Context, key string, value []byte, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
}

const operationTimeout = 5 * time.Second

func operationContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), operationTimeout)
}

func newID() (int64, error) {
	maximum := new(big.Int).SetInt64(1<<62 - 1)
	value, err := rand.Int(rand.Reader, maximum)
	if err != nil {
		return 0, fmt.Errorf("generate ID: %w", err)
	}
	return value.Int64() + 1, nil
}

// NewID returns a cryptographically random positive identifier that is safe to
// expose as an opaque JSON string. Domain services use it for matches, cards,
// listings and immutable ledger records.
func NewID() (int64, error) {
	return newID()
}
