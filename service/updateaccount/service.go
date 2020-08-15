package updateaccount

import (
	"errors"
	"strconv"
	"time"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/resources"
	"github.com/akorwash/QuizBattle/service/login"
)

//CreateAccountServices busniess of how to update account
type UpdateAccountServices struct {
	userRepo repository.IUserRepository
}

//NEWMongo busniess of how to update account
func NEW(_repo repository.IUserRepository) *UpdateAccountServices {
	return &UpdateAccountServices{userRepo: _repo}
}

//UpdateUser apply busniess of validation and create user if passed or return error
func (services UpdateAccountServices) UpdateUser(_user resources.UpdateAccountModel) (*resources.UserAccount, error) {
	userentity, err := validateInutes(services.userRepo, _user)
	if err != nil {
		return nil, err
	}

	err = services.userRepo.UpdateUser(*userentity)
	if err != nil {
		return nil, err
	}

	token, err := login.CreateToken(*userentity)
	if err != nil {
		return nil, err
	}
	response := resources.UserAccount{FullName: userentity.Fullname, Username: userentity.Username, MobileNumber: userentity.MobileNumber, Email: userentity.Email, Token: token}

	return &response, nil
}

//validate models that comes from the body when the user hit the apis
//also validate if the user inputes exist before by another users
//return detailed error
func validateInutes(userRepo repository.IUserRepository, _user resources.UpdateAccountModel) (*entites.User, error) {
	user, err := userRepo.GetUserByID(_user.ID)
	if err != nil {
		return nil, err
	}
	user.DayOfBirth = _user.DayOfBirth
	user.MonthOfBirth = _user.MonthOfBirth
	user.YearOfBirth = _user.YearOfBirth
	const RFC3339FullDate = "2006-01-02"
	date := strconv.Itoa(user.YearOfBirth) + "-"

	if user.MonthOfBirth < 10 {
		date = date + "0" + strconv.Itoa(user.MonthOfBirth) + "-"
	} else {
		date = date + strconv.Itoa(user.MonthOfBirth) + "-"
	}

	if user.DayOfBirth < 10 {
		date = date + "0" + strconv.Itoa(user.DayOfBirth)
	} else {
		date = date + strconv.Itoa(user.DayOfBirth)
	}

	_, err = time.Parse(RFC3339FullDate, date)
	if err != nil {
		return nil, errors.New("Date Time wrong")
	}
	return user, nil
}
