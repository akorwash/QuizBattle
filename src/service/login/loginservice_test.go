package login

import (
	"errors"
	"strings"
	"testing"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/service"
)

type loginRepository struct {
	user           *entites.User
	emailLookup    string
	usernameLookup string
	mobileLookup   string
}

func (repo *loginRepository) result() (*entites.User, error) {
	if repo.user == nil {
		return nil, repository.ErrNotFound
	}
	copy := *repo.user
	return &copy, nil
}

func (repo *loginRepository) GetUserByName(name string) (*entites.User, error) {
	repo.usernameLookup = name
	return repo.result()
}

func (repo *loginRepository) GetUserByMobile(mobile string) (*entites.User, error) {
	repo.mobileLookup = mobile
	return repo.result()
}

func (repo *loginRepository) GetUserByEmail(email string) (*entites.User, error) {
	repo.emailLookup = email
	return repo.result()
}

func (repo *loginRepository) GetUserByID(int64) (*entites.User, error) {
	return repo.result()
}

func (repo *loginRepository) GetUsersByIDs([]int64) (map[int64]entites.User, error) {
	return map[int64]entites.User{}, nil
}

func (repo *loginRepository) AddUser(*entites.User) error   { return nil }
func (repo *loginRepository) UpdateUser(entites.User) error { return nil }

func TestAuthenticateNormalizesIdentifierAndChecksPassword(t *testing.T) {
	hash, err := entites.HashPassword("Strong-pass9!")
	if err != nil {
		t.Fatal(err)
	}
	repo := &loginRepository{user: &entites.User{ID: 9, Username: "player", HashedPassword: hash}}
	user, err := New(repo).Authenticate(" PLAYER@EXAMPLE.COM ", "Strong-pass9!")
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != 9 || repo.emailLookup != "player@example.com" {
		t.Fatalf("unexpected authentication result: %#v, lookup %q", user, repo.emailLookup)
	}
}

func TestAuthenticateUsesGenericFailureForMissingOrWrongPassword(t *testing.T) {
	missing := &loginRepository{}
	if _, err := New(missing).Authenticate("missing", "Strong-pass9!"); !errors.Is(err, service.ErrInvalidCredentials) {
		t.Fatalf("missing account returned %v", err)
	}

	hash, err := entites.HashPassword("Strong-pass9!")
	if err != nil {
		t.Fatal(err)
	}
	existing := &loginRepository{user: &entites.User{ID: 9, Username: "player", HashedPassword: hash}}
	if _, err := New(existing).Authenticate("player", "Wrong-pass9!"); !errors.Is(err, service.ErrInvalidCredentials) {
		t.Fatalf("wrong password returned %v", err)
	}
}

func TestAuthenticateRejectsOversizedIdentifierBeforeRepositoryLookup(t *testing.T) {
	repo := &loginRepository{}
	if _, err := New(repo).Authenticate(strings.Repeat("a", 255), "Strong-pass9!"); !errors.Is(err, service.ErrInvalidCredentials) {
		t.Fatalf("oversized identifier returned %v", err)
	}
	if repo.usernameLookup != "" || repo.emailLookup != "" || repo.mobileLookup != "" {
		t.Fatal("oversized identifier reached the repository")
	}
}
