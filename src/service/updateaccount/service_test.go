package updateaccount

import (
	"testing"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/resources"
)

type updateRepository struct {
	users   map[int64]entites.User
	updated *entites.User
}

func (repo *updateRepository) GetUserByName(string) (*entites.User, error) {
	return nil, repository.ErrNotFound
}

func (repo *updateRepository) GetUserByMobile(string) (*entites.User, error) {
	return nil, repository.ErrNotFound
}

func (repo *updateRepository) GetUserByEmail(string) (*entites.User, error) {
	return nil, repository.ErrNotFound
}

func (repo *updateRepository) GetUserByID(id int64) (*entites.User, error) {
	user, ok := repo.users[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	copy := user
	return &copy, nil
}

func (repo *updateRepository) GetUsersByIDs(ids []int64) (map[int64]entites.User, error) {
	result := make(map[int64]entites.User, len(ids))
	for _, id := range ids {
		if user, ok := repo.users[id]; ok {
			result[id] = user
		}
	}
	return result, nil
}

func (repo *updateRepository) AddUser(*entites.User) error { return nil }

func (repo *updateRepository) UpdateUser(user entites.User) error {
	copy := user
	repo.updated = &copy
	return nil
}

func TestUpdateUserTargetsAuthenticatedUserID(t *testing.T) {
	repo := &updateRepository{users: map[int64]entites.User{
		42: {ID: 42, Username: "authenticated", Fullname: "Old Name"},
		99: {ID: 99, Username: "victim", Fullname: "Victim"},
	}}
	account, err := New(repo).UpdateUser(42, resources.UpdateAccountModel{
		FullName:     "New Name",
		YearOfBirth:  2000,
		MonthOfBirth: 5,
		DayOfBirth:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.updated == nil || repo.updated.ID != 42 || account.UserID != 42 {
		t.Fatalf("updated wrong account: %#v %#v", repo.updated, account)
	}
	if repo.users[99].Fullname != "Victim" {
		t.Fatal("unrelated account was modified")
	}
}
