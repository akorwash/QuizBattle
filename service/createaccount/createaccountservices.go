package createaccount

import (
	"fmt"

	"github.com/akorwash/QuizBattle/datastore"
	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/handler"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/resources"
	"github.com/akorwash/QuizBattle/service/login"
)

//ICreateAccountServices to do
type ICreateAccountServices interface {
	CrateUser(user entites.User) (*resources.UserAccount, error)
}

//CreateAccountServices to do
type CreateAccountServices struct {
}

//CrateUser to do
func (services CreateAccountServices) CrateUser(_user entites.User) (*resources.UserAccount, error) {
	err := validateInutes(_user)
	if err != nil {
		return nil, err
	}

	userentity := entites.User{Username: _user.Username, Password: _user.Password, Email: _user.Email, MobileNumber: _user.MobileNumber}
	err = datastore.MyDBContext.AddUser(userentity)
	if err != nil {
		return nil, err
	}

	token, err := login.CreateToken(userentity)
	if err != nil {
		return nil, err
	}
	response := resources.UserAccount{Username: userentity.Username, MobileNumber: userentity.MobileNumber, Email: userentity.Email, Token: token}

	return &response, nil
}

func validateInutes(_user entites.User) error {
	var userRepo repository.UserRepository

	var usernameValidation handler.ValidateUsername
	if !usernameValidation.Validate(_user.Username) {
		return fmt.Errorf("usernane can't start with numbers, or have a whitespace, should be >= 5 char")
	}

	var mobileValidation handler.ValidateMobile
	if !mobileValidation.Validate(_user.MobileNumber) {
		return fmt.Errorf("mobileNumber is wrong")
	}

	var emailValidation handler.ValidateEmail
	if !emailValidation.Validate(_user.Email) {
		return fmt.Errorf("email is wrong")
	}

	var passwordValidation handler.ValidatePassword
	if !passwordValidation.Validate(_user.Password) {
		return fmt.Errorf("password at least one (upper and lower) case letter, at least one (digit and special) character and should be >= 8 char")
	}

	user, errRepo := userRepo.GetUserByName(_user.Username)
	if errRepo != nil {
		return errRepo
	}

	if user != nil {
		return fmt.Errorf("Username found")
	}

	user, errRepo = userRepo.GetUserByEmail(_user.Email)
	if errRepo != nil {
		return errRepo
	}

	if user != nil {
		return fmt.Errorf("Email found")
	}

	user, errRepo = userRepo.GetUserByMobile(_user.MobileNumber)
	if errRepo != nil {
		return errRepo
	}

	if user != nil {
		return fmt.Errorf("MobileNumber found")
	}

	return nil
}
