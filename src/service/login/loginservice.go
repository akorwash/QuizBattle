package login

import (
	"errors"
	"strings"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/service"
)

// LoginService login services
type LoginService struct {
	userRepo repository.IUserRepository
}

var dummyPasswordHash = func() string {
	hash, err := entites.HashPassword("QuizBattle-dummy-9!")
	if err != nil {
		panic(err)
	}
	return hash
}()

// New create instance for Login services
func New(repository repository.IUserRepository) *LoginService {
	return &LoginService{userRepo: repository}
}

// Login here user can login
func (loginService *LoginService) Authenticate(identifier, password string) (*entites.User, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || len(identifier) > 254 || len(password) == 0 || len(password) > 72 {
		return nil, service.ErrInvalidCredentials
	}
	identifier = strings.ToLower(identifier)

	var (
		user *entites.User
		err  error
	)
	switch {
	case strings.Contains(identifier, "@"):
		user, err = loginService.userRepo.GetUserByEmail(strings.ToLower(identifier))
	case strings.HasPrefix(identifier, "+") || identifier[0] >= '0' && identifier[0] <= '9':
		user, err = loginService.userRepo.GetUserByMobile(identifier)
	default:
		user, err = loginService.userRepo.GetUserByName(strings.ToLower(identifier))
	}
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// Perform the same expensive password check for missing accounts to
			// reduce the timing signal that could otherwise enumerate users.
			_ = (&entites.User{HashedPassword: dummyPasswordHash}).ValidatePassword(password)
			return nil, service.ErrInvalidCredentials
		}
		return nil, err
	}
	if user == nil || !user.ValidatePassword(password) {
		return nil, service.ErrInvalidCredentials
	}
	return user, nil
}
