package createaccount

import (
	"errors"
	"testing"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/resources"
	"github.com/akorwash/QuizBattle/service"
	"golang.org/x/crypto/bcrypt"
)

type accountRepository struct {
	added  *entites.User
	addErr error
}

func (repo *accountRepository) GetUserByName(string) (*entites.User, error) {
	return nil, repository.ErrNotFound
}

func (repo *accountRepository) GetUserByMobile(string) (*entites.User, error) {
	return nil, repository.ErrNotFound
}

func (repo *accountRepository) GetUserByEmail(string) (*entites.User, error) {
	return nil, repository.ErrNotFound
}

func (repo *accountRepository) GetUserByID(int64) (*entites.User, error) {
	return nil, repository.ErrNotFound
}

func (repo *accountRepository) GetUsersByIDs([]int64) (map[int64]entites.User, error) {
	return map[int64]entites.User{}, nil
}

func (repo *accountRepository) AddUser(user *entites.User) error {
	if repo.addErr != nil {
		return repo.addErr
	}
	user.ID = 456
	copy := *user
	repo.added = &copy
	return nil
}

func (repo *accountRepository) UpdateUser(entites.User) error { return nil }

func validAccountInput() resources.CreateAccountModel {
	return resources.CreateAccountModel{
		Username:     " Player_1 ",
		FullName:     " Player One ",
		MobileNumber: "+201000000001",
		Email:        " Player@Example.com ",
		Password:     "Strong-pass9!",
	}
}

func TestCreateUserReturnsPersistedIDAndStrongPasswordHash(t *testing.T) {
	repo := &accountRepository{}
	account, err := New(repo).CreateUser(validAccountInput())
	if err != nil {
		t.Fatal(err)
	}
	if account.UserID != 456 {
		t.Fatalf("expected persisted ID, got %d", account.UserID)
	}
	if account.Username != "player_1" || account.Email != "player@example.com" {
		t.Fatalf("account was not normalized: %#v", account)
	}
	if repo.added == nil || repo.added.HashedPassword == validAccountInput().Password {
		t.Fatal("password was not hashed before persistence")
	}
	cost, err := bcrypt.Cost([]byte(repo.added.HashedPassword))
	if err != nil {
		t.Fatal(err)
	}
	if cost != bcrypt.DefaultCost {
		t.Fatalf("expected bcrypt cost %d, got %d", bcrypt.DefaultCost, cost)
	}
}

func TestCreateUserMapsInsertRaceToAccountConflict(t *testing.T) {
	repo := &accountRepository{addErr: repository.ErrConflict}
	_, err := New(repo).CreateUser(validAccountInput())
	if !errors.Is(err, service.ErrAccountExists) {
		t.Fatalf("expected account conflict, got %v", err)
	}
}
