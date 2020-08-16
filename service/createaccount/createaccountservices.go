package createaccount

import (
	"fmt"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/handler"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/resources"
	"github.com/akorwash/QuizBattle/service/login"
)

//CreateAccountServices busniess of how to create account
type CreateAccountServices struct {
	userRepo repository.IUserRepository
}

//NEWMongo busniess of how to create account
func NEW(_repo repository.IUserRepository) *CreateAccountServices {
	return &CreateAccountServices{userRepo: _repo}
}

//CrateUser apply busniess of validation and create user if passed or return error
func (services CreateAccountServices) CrateUser(_user resources.CreateAccountModel) (*resources.UserAccount, error) {
	err := validateInutes(services.userRepo, _user)
	if err != nil {
		return nil, err
	}

	userentity := entites.User{Fullname: _user.FullName, Username: _user.Username, HashedPassword: entites.HashAndSalt([]byte(_user.Password)), Email: _user.Email, MobileNumber: _user.MobileNumber}
	err = services.userRepo.AddUser(userentity)
	if err != nil {
		return nil, err
	}

	token, err := login.CreateToken(userentity)
	if err != nil {
		return nil, err
	}
	response := resources.UserAccount{FullName: userentity.Fullname, Username: userentity.Username, MobileNumber: userentity.MobileNumber, Email: userentity.Email, Token: token}

	return &response, nil
}

//validate models that comes from the body when the user hit the apis
//also validate if the user inputes exist before by another users
//return detailed error
func validateInutes(userRepo repository.IUserRepository, _user resources.CreateAccountModel) error {

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
	if errRepo != nil && errRepo.Error() != "User not found" {
		return errRepo
	}

	if user != nil {
		return fmt.Errorf("Username found")
	}

	user, errRepo = userRepo.GetUserByEmail(_user.Email)
	if errRepo != nil && errRepo.Error() != "User not found" {
		return errRepo
	}

	if user != nil {
		return fmt.Errorf("Email found")
	}

	user, errRepo = userRepo.GetUserByMobile(_user.MobileNumber)
	if errRepo != nil && errRepo.Error() != "User not found" {
		return errRepo
	}

	if user != nil {
		return fmt.Errorf("MobileNumber found")
	}

	return nil
}
