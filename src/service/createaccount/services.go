package createaccount

import (
	"errors"
	"fmt"
	"strings"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/handler"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/resources"
	"github.com/akorwash/QuizBattle/service"
)

// CreateAccountServices busniess of how to create account
type CreateAccountServices struct {
	userRepo repository.IUserRepository
}

// NEWMongo busniess of how to create account
func New(_repo repository.IUserRepository) *CreateAccountServices {
	return &CreateAccountServices{userRepo: _repo}
}

// CrateUser apply busniess of validation and create user if passed or return error
func (services CreateAccountServices) CreateUser(_user resources.CreateAccountModel) (*resources.UserAccount, error) {
	_user.Username = strings.ToLower(strings.TrimSpace(_user.Username))
	_user.FullName = strings.TrimSpace(_user.FullName)
	_user.Email = strings.ToLower(strings.TrimSpace(_user.Email))
	_user.MobileNumber = strings.TrimSpace(_user.MobileNumber)

	err := validateInputs(_user)
	if err != nil {
		return nil, err
	}

	hashedPassword, err := entites.HashPassword(_user.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	userEntity := entites.User{Fullname: _user.FullName, Username: _user.Username, HashedPassword: hashedPassword, Email: _user.Email, MobileNumber: _user.MobileNumber}
	err = services.userRepo.AddUser(&userEntity)
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, service.ErrAccountExists
		}
		return nil, err
	}
	response := resources.UserAccount{UserID: userEntity.ID, FullName: userEntity.Fullname, Username: userEntity.Username, MobileNumber: userEntity.MobileNumber, Email: userEntity.Email}

	return &response, nil
}

// validate models that comes from the body when the user hit the apis
// also validate if the user inputes exist before by another users
// return detailed error
func validateInputs(_user resources.CreateAccountModel) error {
	if !handler.IsFullNameValid(_user.FullName) {
		return fmt.Errorf("%w: full name must contain between 2 and 80 characters", service.ErrInvalidInput)
	}

	var usernameValidation handler.ValidateUsername
	if !usernameValidation.Validate(_user.Username) {
		return fmt.Errorf("%w: username must start with a letter and contain 5-32 letters, numbers, dots, underscores, or hyphens", service.ErrInvalidInput)
	}

	var mobileValidation handler.ValidateMobile
	if !mobileValidation.Validate(_user.MobileNumber) {
		return fmt.Errorf("%w: mobile number is invalid", service.ErrInvalidInput)
	}

	var emailValidation handler.ValidateEmail
	if !emailValidation.Validate(_user.Email) {
		return fmt.Errorf("%w: email address is invalid", service.ErrInvalidInput)
	}

	var passwordValidation handler.ValidatePassword
	if !passwordValidation.Validate(_user.Password) {
		return fmt.Errorf("%w: password must contain 10-72 characters including upper, lower, digit, and symbol", service.ErrInvalidInput)
	}

	return nil
}
