package updateaccount

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/handler"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/resources"
	"github.com/akorwash/QuizBattle/service"
)

// CreateAccountServices busniess of how to update account
type UpdateAccountServices struct {
	userRepo repository.IUserRepository
}

// NEWMongo busniess of how to update account
func New(_repo repository.IUserRepository) *UpdateAccountServices {
	return &UpdateAccountServices{userRepo: _repo}
}

// UpdateUser apply busniess of validation and create user if passed or return error
func (services UpdateAccountServices) UpdateUser(userID int64, input resources.UpdateAccountModel) (*resources.UserAccount, error) {
	userEntity, err := validateInputs(services.userRepo, userID, input)
	if err != nil {
		return nil, err
	}

	err = services.userRepo.UpdateUser(*userEntity)
	if err != nil {
		return nil, err
	}

	response := resources.UserAccount{
		UserID:       userEntity.ID,
		FullName:     userEntity.Fullname,
		Username:     userEntity.Username,
		MobileNumber: userEntity.MobileNumber,
		Email:        userEntity.Email,
		YearOfBirth:  userEntity.YearOfBirth,
		MonthOfBirth: userEntity.MonthOfBirth,
		DayOfBirth:   userEntity.DayOfBirth,
	}

	return &response, nil
}

// validate models that comes from the body when the user hit the apis
// also validate if the user inputes exist before by another users
// return detailed error
func validateInputs(userRepo repository.IUserRepository, userID int64, input resources.UpdateAccountModel) (*entites.User, error) {
	if userID <= 0 {
		return nil, service.ErrForbidden
	}
	input.FullName = strings.TrimSpace(input.FullName)
	if !handler.IsFullNameValid(input.FullName) {
		return nil, fmt.Errorf("%w: full name must contain between 2 and 80 characters", service.ErrInvalidInput)
	}
	user, err := userRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	birthDate := time.Date(input.YearOfBirth, time.Month(input.MonthOfBirth), input.DayOfBirth, 0, 0, 0, 0, time.UTC)
	if birthDate.Year() != input.YearOfBirth || int(birthDate.Month()) != input.MonthOfBirth || birthDate.Day() != input.DayOfBirth || birthDate.After(time.Now().UTC()) || input.YearOfBirth < 1900 {
		return nil, errors.Join(service.ErrInvalidInput, errors.New("date of birth is invalid"))
	}
	user.Fullname = input.FullName
	user.DayOfBirth = input.DayOfBirth
	user.MonthOfBirth = input.MonthOfBirth
	user.YearOfBirth = input.YearOfBirth
	return user, nil
}
